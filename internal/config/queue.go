package config

import (
	"fmt"
	"net/url"
	"time"
)

type QueueConfig struct {
	URL            string
	Queue          string
	Prefetch       int
	ReconnectDelay time.Duration
}

const (
	envVarQueueURL       = "RABBITMQ_URL"
	envVarQueueName      = "TASK_QUEUE_NAME"
	envVarQueuePrefetch  = "TASK_QUEUE_PREFETCH"
	envVarQueueReconnect = "TASK_QUEUE_RECONNECT_DELAY"
)

const (
	defaultQueueURL       = "amqp://guest:guest@localhost:5672/"
	defaultQueueName      = "tasks"
	defaultQueuePrefetch  = 1
	defaultQueueReconnect = time.Second
)

func LoadQueueConfig() (QueueConfig, error) {
	queueCfg := QueueConfig{
		URL:            envString(envVarQueueURL, defaultQueueURL),
		Queue:          envString(envVarQueueName, defaultQueueName),
		Prefetch:       envInt(envVarQueuePrefetch, defaultQueuePrefetch),
		ReconnectDelay: envDuration(envVarQueueReconnect, defaultQueueReconnect),
	}

	if err := validateQueueConfig(queueCfg); err != nil {
		return QueueConfig{}, err
	}
	return queueCfg, nil
}

func validateQueueConfig(queueCfg QueueConfig) error {
	u, err := url.Parse(queueCfg.URL)
	if err != nil || u.Host == "" || (u.Scheme != "amqp" && u.Scheme != "amqps") {
		return fmt.Errorf("%s must be a valid amqp(s) URL, got %q", envVarQueueURL, queueCfg.URL)
	}
	if queueCfg.Queue == "" {
		return fmt.Errorf("%s must not be empty", envVarQueueName)
	}
	if queueCfg.Prefetch <= 0 {
		return fmt.Errorf("%s must be positive, got %d", envVarQueuePrefetch, queueCfg.Prefetch)
	}
	if queueCfg.ReconnectDelay <= 0 {
		return fmt.Errorf("%s must be positive, got %s", envVarQueueReconnect, queueCfg.ReconnectDelay)
	}
	return nil
}
