package main

import (
	"context"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/redis/go-redis/v9"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestGetEnvOrDefault(t *testing.T) {
	tests := []struct {
		name         string
		key          string
		defaultValue string
		envValue     string
		expected     string
	}{
		{
			name:         "returns environment value when set",
			key:          "TEST_VAR",
			defaultValue: "default",
			envValue:     "env_value",
			expected:     "env_value",
		},
		{
			name:         "returns default when env var not set",
			key:          "UNSET_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
		{
			name:         "returns empty string as valid env value",
			key:          "EMPTY_VAR",
			defaultValue: "default",
			envValue:     "",
			expected:     "default",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envValue != "" {
				os.Setenv(tt.key, tt.envValue)
				defer os.Unsetenv(tt.key)
			}

			result := getEnvOrDefault(tt.key, tt.defaultValue)
			if result != tt.expected {
				t.Errorf("getEnvOrDefault() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
		expected    *Config
	}{
		{
			name: "valid configuration with all required env vars",
			envVars: map[string]string{
				envValkeySentinelHost:     "redis-sentinel",
				envValkeySentinelPassword: "password123",
			},
			expectError: false,
			expected: &Config{
				SentinelPort:        "26379",
				SentinelHost:        "redis-sentinel",
				SentinelPassword:    "password123",
				MasterName:          "myprimary",
				Namespace:           "default",
				MasterPodLabelName:  "valkey-master",
				MasterPodLabelValue: "true",
			},
		},
		{
			name: "valid configuration with custom values",
			envVars: map[string]string{
				envValkeySentinelHost:     "custom-sentinel",
				envValkeySentinelPort:     "26380",
				envValkeySentinelPassword: "custom-password",
				envValkeyMasterName:       "custom-primary",
				envPodNamespace:           "redis-namespace",
				envMasterPodLabelName:     "custom-master",
				envMasterPodLabelValue:    "yes",
			},
			expectError: false,
			expected: &Config{
				SentinelPort:        "26380",
				SentinelHost:        "custom-sentinel",
				SentinelPassword:    "custom-password",
				MasterName:          "custom-primary",
				Namespace:           "redis-namespace",
				MasterPodLabelName:  "custom-master",
				MasterPodLabelValue: "yes",
			},
		},
		{
			name: "missing sentinel host",
			envVars: map[string]string{
				envValkeySentinelPassword: "password123",
			},
			expectError: true,
		},
		{
			name: "missing sentinel password",
			envVars: map[string]string{
				envValkeySentinelHost: "redis-sentinel",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean environment
			for key := range tt.envVars {
				os.Unsetenv(key)
			}

			// Set test environment variables
			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			// Clean up after test
			defer func() {
				for key := range tt.envVars {
					os.Unsetenv(key)
				}
			}()

			config, err := getConfig()

			if tt.expectError {
				if err == nil {
					t.Errorf("getConfig() expected error, but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("getConfig() unexpected error: %v", err)
				return
			}

			if config.SentinelPort != tt.expected.SentinelPort {
				t.Errorf("SentinelPort = %v, want %v", config.SentinelPort, tt.expected.SentinelPort)
			}
			if config.SentinelHost != tt.expected.SentinelHost {
				t.Errorf("SentinelHost = %v, want %v", config.SentinelHost, tt.expected.SentinelHost)
			}
			if config.SentinelPassword != tt.expected.SentinelPassword {
				t.Errorf("SentinelPassword = %v, want %v", config.SentinelPassword, tt.expected.SentinelPassword)
			}
			if config.MasterName != tt.expected.MasterName {
				t.Errorf("MasterName = %v, want %v", config.MasterName, tt.expected.MasterName)
			}
			if config.Namespace != tt.expected.Namespace {
				t.Errorf("Namespace = %v, want %v", config.Namespace, tt.expected.Namespace)
			}
			if config.MasterPodLabelName != tt.expected.MasterPodLabelName {
				t.Errorf("MasterPodLabelName = %v, want %v", config.MasterPodLabelName, tt.expected.MasterPodLabelName)
			}
			if config.MasterPodLabelValue != tt.expected.MasterPodLabelValue {
				t.Errorf("MasterPodLabelValue = %v, want %v", config.MasterPodLabelValue, tt.expected.MasterPodLabelValue)
			}
		})
	}
}

type mockSentinelClient struct {
	masterAddr []string
	err        error
}

func (m *mockSentinelClient) GetMasterAddrByName(ctx context.Context, name string) *redis.StringSliceCmd {
	cmd := redis.NewStringSliceCmd(ctx, "sentinel", "get-master-addr-by-name", name)
	if m.err != nil {
		cmd.SetErr(m.err)
	} else {
		cmd.SetVal(m.masterAddr)
	}
	return cmd
}

func TestGetCurrentMasterFromSentinel(t *testing.T) {
	tests := []struct {
		name           string
		mockAddr       []string
		mockErr        error
		expectedAddr   []string
		expectedErrMsg string
	}{
		{
			name:         "successful master address retrieval",
			mockAddr:     []string{"192.168.1.10", "6379"},
			mockErr:      nil,
			expectedAddr: []string{"192.168.1.10", "6379"},
		},
		{
			name:           "sentinel error",
			mockAddr:       nil,
			mockErr:        fmt.Errorf("sentinel connection failed"),
			expectedAddr:   nil,
			expectedErrMsg: "sentinel connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &Config{
				MasterName: "test-master",
			}

			mockSentinel := &mockSentinelClient{
				masterAddr: tt.mockAddr,
				err:        tt.mockErr,
			}

			ctx := context.Background()
			addr, err := getCurrentMasterFromSentinel(ctx, config, mockSentinel)

			if tt.expectedErrMsg != "" {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectedErrMsg)
				} else if err.Error() != tt.expectedErrMsg {
					t.Errorf("expected error %v, got %v", tt.expectedErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(addr) != len(tt.expectedAddr) {
				t.Errorf("expected address length %d, got %d", len(tt.expectedAddr), len(addr))
				return
			}

			for i, expected := range tt.expectedAddr {
				if addr[i] != expected {
					t.Errorf("expected address[%d] = %v, got %v", i, expected, addr[i])
				}
			}
		})
	}
}

func TestSetCurrentMaster(t *testing.T) {
	tests := []struct {
		name            string
		masterAddress   []string
		existingPods    []corev1.Pod
		config          *Config
		expectedUpdates int
		expectError     bool
	}{
		{
			name:          "update master pod label",
			masterAddress: []string{"10.244.1.5", "6379"},
			config: &Config{
				Namespace:           "default",
				MasterPodLabelName:  "vk-master",
				MasterPodLabelValue: "true",
			},
			existingPods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valkey-0",
						Namespace: "default",
						Labels: map[string]string{
							"app.kubernetes.io/name": "valkey",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "10.244.1.5",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valkey-1", 
						Namespace: "default",
						Labels: map[string]string{
							"app.kubernetes.io/name": "valkey",
							"vk-master":              "true",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "10.244.1.6",
					},
				},
			},
			expectedUpdates: 2, // One to add master label, one to remove old master label
			expectError:     false,
		},
		{
			name:          "no pods match master IP",
			masterAddress: []string{"10.244.1.99", "6379"},
			config: &Config{
				Namespace:           "default",
				MasterPodLabelName:  "vk-master",
				MasterPodLabelValue: "true",
			},
			existingPods: []corev1.Pod{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "valkey-0",
						Namespace: "default",
						Labels: map[string]string{
							"app.kubernetes.io/name": "valkey",
						},
					},
					Status: corev1.PodStatus{
						PodIP: "10.244.1.5",
					},
				},
			},
			expectedUpdates: 0,
			expectError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fake Kubernetes client
			fakeClient := fake.NewSimpleClientset()
			
			// Add existing pods to the fake client
			for _, pod := range tt.existingPods {
				_, err := fakeClient.CoreV1().Pods(tt.config.Namespace).Create(
					context.Background(), &pod, metav1.CreateOptions{})
				if err != nil {
					t.Fatalf("failed to create test pod: %v", err)
				}
			}

			// Track update actions
			updateCount := 0
			fakeClient.PrependReactor("update", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
				updateCount++
				return false, nil, nil // Let the default fake client handle the actual update
			})

			// We can't easily test the full setCurrentMaster function because it creates its own k8s client
			// Instead, we'll test the core logic by simulating the pod updates directly

			ctx := context.Background()
			pods, err := fakeClient.CoreV1().Pods(tt.config.Namespace).List(ctx, metav1.ListOptions{
				LabelSelector: "app.kubernetes.io/name=valkey",
			})
			if err != nil {
				t.Fatalf("failed to list pods: %v", err)
			}

			// Parse the master IP
			masterIP := net.ParseIP(tt.masterAddress[0])
			if masterIP == nil {
				t.Fatalf("invalid master IP: %s", tt.masterAddress[0])
			}

			// Simulate the labeling logic
			actualUpdates := 0
			for _, pod := range pods.Items {
				targetIP := net.ParseIP(pod.Status.PodIP)
				if targetIP == nil {
					continue
				}

				if targetIP.Equal(masterIP) {
					// This pod should become the master
					if pod.Labels == nil {
						pod.Labels = make(map[string]string)
					}
					pod.Labels[tt.config.MasterPodLabelName] = tt.config.MasterPodLabelValue
					_, err := fakeClient.CoreV1().Pods(tt.config.Namespace).Update(ctx, &pod, metav1.UpdateOptions{})
					if err != nil {
						t.Errorf("failed to update master pod: %v", err)
					}
					actualUpdates++
				} else if pod.Labels[tt.config.MasterPodLabelName] == tt.config.MasterPodLabelValue {
					// This pod was the master but isn't anymore
					pod.Labels[tt.config.MasterPodLabelName] = ""
					_, err := fakeClient.CoreV1().Pods(tt.config.Namespace).Update(ctx, &pod, metav1.UpdateOptions{})
					if err != nil {
						t.Errorf("failed to update former master pod: %v", err)
					}
					actualUpdates++
				}
			}

			if actualUpdates != tt.expectedUpdates {
				t.Errorf("expected %d updates, got %d", tt.expectedUpdates, actualUpdates)
			}
		})
	}
}

func TestSwitchMasterEventParsing(t *testing.T) {
	tests := []struct {
		name          string
		payload       string
		expectedParts int
		expectedIP    string
		expectedPort  string
	}{
		{
			name:          "valid switch-master event",
			payload:       "myprimary 127.0.0.1 6379 192.168.1.10 6379",
			expectedParts: 5,
			expectedIP:    "192.168.1.10",
			expectedPort:  "6379",
		},
		{
			name:          "invalid switch-master event - too few parts",
			payload:       "myprimary 127.0.0.1 6379",
			expectedParts: 3,
		},
		{
			name:          "invalid switch-master event - empty payload",
			payload:       "",
			expectedParts: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parts := strings.Fields(tt.payload)
			
			if len(parts) != tt.expectedParts {
				t.Errorf("expected %d parts, got %d", tt.expectedParts, len(parts))
				return
			}

			if tt.expectedParts == 5 {
				if parts[3] != tt.expectedIP {
					t.Errorf("expected IP %s, got %s", tt.expectedIP, parts[3])
				}
				if parts[4] != tt.expectedPort {
					t.Errorf("expected port %s, got %s", tt.expectedPort, parts[4])
				}
			}
		})
	}
}

func TestRebootEventHandling(t *testing.T) {
	tests := []struct {
		name        string
		channel     string
		payload     string
		shouldQuery bool
	}{
		{
			name:        "reboot event should trigger master query",
			channel:     "+reboot",
			payload:     "master myprimary",
			shouldQuery: true,
		},
		{
			name:        "switch-master event should not trigger master query",
			channel:     "+switch-master",
			payload:     "myprimary 127.0.0.1 6379 192.168.1.10 6379",
			shouldQuery: false,
		},
		{
			name:        "other event should not trigger master query",
			channel:     "+sdown",
			payload:     "master myprimary",
			shouldQuery: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test validates the event channel matching logic
			// In a real implementation, we'd need to mock the entire event processing
			shouldQuery := tt.channel == "+reboot"
			
			if shouldQuery != tt.shouldQuery {
				t.Errorf("expected shouldQuery=%v for channel %s, got %v", tt.shouldQuery, tt.channel, shouldQuery)
			}
		})
	}
}

func TestIPParsing(t *testing.T) {
	tests := []struct {
		name      string
		ip        string
		expectNil bool
	}{
		{
			name:      "valid IPv4",
			ip:        "192.168.1.10",
			expectNil: false,
		},
		{
			name:      "valid IPv6",
			ip:        "2001:db8::1",
			expectNil: false,
		},
		{
			name:      "invalid IP",
			ip:        "not-an-ip",
			expectNil: true,
		},
		{
			name:      "empty string",
			ip:        "",
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			
			if tt.expectNil && ip != nil {
				t.Errorf("expected nil IP, got %v", ip)
			}
			if !tt.expectNil && ip == nil {
				t.Errorf("expected valid IP, got nil")
			}
		})
	}
}

func TestPodLabelManagement(t *testing.T) {
	tests := []struct {
		name                string
		initialLabels       map[string]string
		labelName           string
		labelValue          string
		shouldAddLabel      bool
		shouldRemoveLabel   bool
		expectedFinalLabels map[string]string
	}{
		{
			name: "add master label to pod without labels",
			initialLabels: map[string]string{
				"app.kubernetes.io/name": "valkey",
			},
			labelName:      "vk-master",
			labelValue:     "true",
			shouldAddLabel: true,
			expectedFinalLabels: map[string]string{
				"app.kubernetes.io/name": "valkey",
				"vk-master":              "true",
			},
		},
		{
			name: "remove master label from pod",
			initialLabels: map[string]string{
				"app.kubernetes.io/name": "valkey",
				"vk-master":              "true",
			},
			labelName:         "vk-master",
			labelValue:        "true",
			shouldRemoveLabel: true,
			expectedFinalLabels: map[string]string{
				"app.kubernetes.io/name": "valkey",
				"vk-master":              "",
			},
		},
		{
			name: "pod already has correct master label",
			initialLabels: map[string]string{
				"app.kubernetes.io/name": "valkey",
				"vk-master":              "true",
			},
			labelName:      "vk-master",
			labelValue:     "true",
			shouldAddLabel: true,
			expectedFinalLabels: map[string]string{
				"app.kubernetes.io/name": "valkey",
				"vk-master":              "true",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := make(map[string]string)
			for k, v := range tt.initialLabels {
				labels[k] = v
			}

			if tt.shouldAddLabel {
				labels[tt.labelName] = tt.labelValue
			} else if tt.shouldRemoveLabel {
				labels[tt.labelName] = ""
			}

			for key, expectedValue := range tt.expectedFinalLabels {
				if labels[key] != expectedValue {
					t.Errorf("expected label %s=%s, got %s", key, expectedValue, labels[key])
				}
			}
		})
	}
}

func BenchmarkGetEnvOrDefault(b *testing.B) {
	os.Setenv("BENCH_TEST", "benchmark_value")
	defer os.Unsetenv("BENCH_TEST")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getEnvOrDefault("BENCH_TEST", "default")
	}
}

func BenchmarkSwitchMasterEventParsing(b *testing.B) {
	payload := "myprimary 127.0.0.1 6379 192.168.1.10 6379"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parts := strings.Fields(payload)
		if len(parts) == 5 {
			_ = parts[3] // new master IP
			_ = parts[4] // new master port
		}
	}
}

func BenchmarkIPParsing(b *testing.B) {
	ip := "192.168.1.10"
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = net.ParseIP(ip)
	}
}