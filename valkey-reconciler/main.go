package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// SentinelClient interface for testing
type SentinelClient interface {
	GetMasterAddrByName(ctx context.Context, name string) *redis.StringSliceCmd
}

const (
	// Environment variables
	envValkeySentinelPort     = "VALKEY_SENTINEL_PORT"
	envValkeySentinelHost     = "VALKEY_SENTINEL_HOST"
	envValkeySentinelPassword = "VALKEY_SENTINEL_PASSWORD"
	envValkeyMasterName       = "VALKEY_MASTER_NAME"
	envPodNamespace           = "POD_NAMESPACE"
	envMasterPodLabelName     = "MASTER_POD_LABEL_NAME"
	envMasterPodLabelValue    = "MASTER_POD_LABEL_VALUE"
)

type Config struct {
	SentinelPort        string
	SentinelHost        string
	SentinelPassword    string
	ServiceName         string
	MasterName          string
	Namespace           string
	MasterPodLabelName  string
	MasterPodLabelValue string
}

func getConfig() (*Config, error) {
	config := &Config{
		SentinelPort:        getEnvOrDefault(envValkeySentinelPort, "26379"),
		SentinelHost:        getEnvOrDefault(envValkeySentinelHost, ""),
		SentinelPassword:    getEnvOrDefault(envValkeySentinelPassword, ""),
		MasterName:          getEnvOrDefault(envValkeyMasterName, "myprimary"),
		Namespace:           getEnvOrDefault(envPodNamespace, "default"),
		MasterPodLabelName:  getEnvOrDefault(envMasterPodLabelName, "valkey-master"),
		MasterPodLabelValue: getEnvOrDefault(envMasterPodLabelValue, "true"),
	}

	if config.SentinelHost == "" {
		return nil, fmt.Errorf("%s environment variable is required", envValkeySentinelHost)
	}

	if config.SentinelPassword == "" {
		return nil, fmt.Errorf("%s environment variable is required", envValkeySentinelPassword)
	}

	return config, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getCurrentMaster(ctx context.Context, config *Config) ([]string, error) {
	sentinel := redis.NewSentinelClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%s", config.SentinelHost, config.SentinelPort),
		Password: config.SentinelPassword,
		TLSConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	})

	log.Printf("Searching for current master at host: %s, port: %s, master name: %s", config.SentinelHost, config.SentinelPort, config.MasterName)
	return getCurrentMasterFromSentinel(ctx, config, sentinel)
}

func getCurrentMasterFromSentinel(ctx context.Context, config *Config, sentinel SentinelClient) ([]string, error) {
	masterAddress, err := sentinel.GetMasterAddrByName(ctx, config.MasterName).Result()

	if err != nil {
		log.Printf("Failed to get current master: %v", err)
		return masterAddress, err
	}

	return masterAddress, nil
}


func setCurrentMaster(ctx context.Context, config *Config, masterAddress []string) {

	masterIp, err := net.LookupIP(masterAddress[0])
	if err != nil {
		log.Fatalf("Failed to lookup master IP: %v", err)
	}

	// Create Kubernetes client
	k8sConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("Failed to get Kubernetes config: %v", err)
	}

	clientset, err := kubernetes.NewForConfig(k8sConfig)
	if err != nil {
		log.Fatalf("Failed to create Kubernetes client: %v", err)
	}

	log.Printf("Setting current master to %s:%s", masterAddress[0], masterAddress[1])

	pods, err := clientset.CoreV1().Pods(config.Namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: "app.kubernetes.io/name=valkey",
	})

	if err != nil {
		log.Fatalf("Failed to list pods: %v", err)
	}

	log.Printf("Found %d pods with label app.kubernetes.io/name=valkey", len(pods.Items))

	for _, pod := range pods.Items {

		targetIP := net.ParseIP(pod.Status.PodIP)
		if targetIP == nil {
			log.Printf("Invalid pod IP: %s", pod.Status.PodIP)
			continue
		}
		if targetIP.Equal(masterIp[0]) {
			log.Printf("Pod %s is the master", pod.Name)
			pod.Labels[config.MasterPodLabelName] = config.MasterPodLabelValue
			_, err := clientset.CoreV1().Pods(config.Namespace).Update(context.Background(), &pod, metav1.UpdateOptions{})
			if err != nil {
				log.Printf("Failed to label pod %s as master: %v", pod.Name, err)
				continue
			}
		} else if pod.Labels[config.MasterPodLabelName] == config.MasterPodLabelValue {
			log.Printf("Pod %s was the master, removing label", pod.Name)
			pod.Labels[config.MasterPodLabelName] = ""
			_, err := clientset.CoreV1().Pods(config.Namespace).Update(context.Background(), &pod, metav1.UpdateOptions{})
			if err != nil {
				log.Printf("Failed to remove label from pod %s: %v", pod.Name, err)
				continue
			}
		} else {
			log.Printf("Pod %s is not the master", pod.Name)
		}
	}

}

func listenForSwitchMasterEvents(ctx context.Context, config *Config, currentMaster []string) {

	for {

		log.Printf("Connecting to sentinel at %s:%s", config.SentinelHost, config.SentinelPort)
		sentinel := redis.NewSentinelClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%s", config.SentinelHost, config.SentinelPort),
			Password: config.SentinelPassword,
			TLSConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			MaxRetries:  -1,
			ReadTimeout: 1 * time.Second,
			OnConnect: func(ctx context.Context, cn *redis.Conn) error {
				log.Printf("Connection established")

				currentMaster, err := getCurrentMaster(ctx, config)
				if err != nil {
					log.Printf("Failed to get current master: %v", err)
					return nil
				}

				log.Printf("Current master: %v", currentMaster)

				setCurrentMaster(ctx, config, currentMaster)

				return nil
			},
		})

		_, pingErr := sentinel.Ping(ctx).Result()
		if pingErr != nil {
			log.Printf("Failed to ping sentinel: %v", pingErr)
			time.Sleep(1 * time.Second)
			continue
		}

		log.Printf("Listening for switch-master events")

		pubsub := sentinel.PSubscribe(ctx, "*")

		_, err := pubsub.Receive(ctx)
		if err != nil {
			log.Fatalf("Failed to receive event: %v", err)
			continue
		}

		log.Printf("Subscribed to switch-master events")

		// Consume messages.
		for msg := range pubsub.Channel(redis.WithChannelHealthCheckInterval(1 * time.Second)) {
			log.Printf("Received %s message %s", msg.Channel, msg.Payload)

			if msg.Channel == "+switch-master" {
				parts := strings.Fields(msg.Payload)
				if len(parts) != 5 {
					log.Printf("Invalid switch-master event format: %d", len(parts))
					for _, line := range parts {
						log.Printf("\tLine: %s", line)
					}
					continue
				}
				setCurrentMaster(ctx, config, parts[3:5])
			} else if msg.Channel == "+reboot" {
				log.Printf("Received reboot event, fetching current master")
				currentMaster, err := getCurrentMaster(ctx, config)
				if err != nil {
					log.Printf("Failed to get current master after reboot event: %v", err)
					continue
				}
				log.Printf("Current master after reboot: %v", currentMaster)
				setCurrentMaster(ctx, config, currentMaster)
			} else {
				// log.Printf("Received %s message %s", msg.Channel, msg.Payload)
			}
		}

		pubsub.Close()
		sentinel.Close()
		log.Printf("Connection to sentinel lost, reconnecting")

		time.Sleep(2 * time.Second)
	}
}

func main() {
	config, err := getConfig()
	ctx := context.Background()
	if err != nil {
		log.Fatalf("Failed to get configuration: %v", err)
	}

	currentMaster, err := getCurrentMaster(ctx, config)
	if err != nil {
		log.Fatalf("Failed to get current master: %v", err)
	}

	log.Printf("Current master: %v", currentMaster)

	setCurrentMaster(ctx, config, currentMaster)

	listenForSwitchMasterEvents(ctx, config, currentMaster)

}
