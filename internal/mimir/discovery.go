// internal/mimir/discovery.go
package mimir

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/seanankenbruck/observability-ai/internal/semantic"
)

// DiscoveryConfig holds configuration for service discovery
type DiscoveryConfig struct {
	Enabled           bool
	Interval          time.Duration
	Namespaces        []string
	ServiceLabelNames []string
	ExcludeMetrics    []string
}

// DiscoveredService represents a service discovered from metrics
type DiscoveredService struct {
	Name      string
	Namespace string
	Labels    map[string]string
	Metrics   []string
}

// DiscoveryService automatically discovers services and metrics from Mimir
type DiscoveryService struct {
	client         *Client
	config         DiscoveryConfig
	mapper         semantic.Mapper
	stopChan       chan struct{}
	ticker         *time.Ticker
	running        bool
	mu             sync.Mutex
	excludePatterns []*regexp.Regexp
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(client *Client, config DiscoveryConfig, mapper semantic.Mapper) *DiscoveryService {
	// Set defaults
	if config.Interval == 0 {
		config.Interval = 5 * time.Minute
	}
	if len(config.ServiceLabelNames) == 0 {
		config.ServiceLabelNames = []string{"service", "job", "app", "application"}
	}

	// Compile exclude patterns
	var excludePatterns []*regexp.Regexp
	for _, pattern := range config.ExcludeMetrics {
		if re, err := regexp.Compile(pattern); err == nil {
			excludePatterns = append(excludePatterns, re)
		} else {
			log.Printf("Warning: Invalid exclude pattern %s: %v", pattern, err)
		}
	}

	return &DiscoveryService{
		client:          client,
		config:          config,
		mapper:          mapper,
		stopChan:        make(chan struct{}),
		excludePatterns: excludePatterns,
	}
}

// Start begins periodic service discovery
func (ds *DiscoveryService) Start(ctx context.Context) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if ds.running {
		return fmt.Errorf("discovery service already running")
	}

	if !ds.config.Enabled {
		log.Println("Service discovery is disabled")
		return nil
	}

	// Test Mimir connection first
	if err := ds.client.TestConnection(ctx); err != nil {
		return fmt.Errorf("failed to connect to Mimir: %w", err)
	}

	log.Println("Mimir connection successful, starting service discovery")

	// Run initial discovery immediately
	go func() {
		if err := ds.runDiscovery(ctx); err != nil {
			log.Printf("Initial discovery error: %v", err)
		}
	}()

	// Start periodic discovery
	ds.ticker = time.NewTicker(ds.config.Interval)
	ds.running = true

	go ds.discoveryLoop(ctx)

	log.Printf("Service discovery started with interval: %v", ds.config.Interval)
	return nil
}

// Stop stops the discovery service
func (ds *DiscoveryService) Stop() {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	if !ds.running {
		return
	}

	close(ds.stopChan)
	if ds.ticker != nil {
		ds.ticker.Stop()
	}
	ds.running = false

	log.Println("Service discovery stopped")
}

// discoveryLoop runs periodic discovery
func (ds *DiscoveryService) discoveryLoop(ctx context.Context) {
	for {
		select {
		case <-ds.stopChan:
			return
		case <-ds.ticker.C:
			if err := ds.runDiscovery(ctx); err != nil {
				log.Printf("Discovery error: %v", err)
			}
		}
	}
}

// runDiscovery performs a single discovery cycle
func (ds *DiscoveryService) runDiscovery(ctx context.Context) error {
	log.Println("Starting service discovery cycle...")
	startTime := time.Now()

	// Fetch all metric names
	metricNames, err := ds.client.GetMetricNames(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch metric names: %w", err)
	}

	log.Printf("Found %d total metrics", len(metricNames))

	// Filter metrics based on exclude patterns
	filteredMetrics := ds.filterMetrics(metricNames)
	log.Printf("Filtered to %d metrics after applying exclusions", len(filteredMetrics))

	// Discover services from metrics
	services, err := ds.discoverServices(ctx, filteredMetrics)
	if err != nil {
		return fmt.Errorf("failed to discover services: %w", err)
	}

	log.Printf("Discovered %d services", len(services))

	// Update database with discovered services
	updates, err := ds.updateDatabase(ctx, services)
	if err != nil {
		return fmt.Errorf("failed to update database: %w", err)
	}

	duration := time.Since(startTime)
	log.Printf("Discovery cycle completed in %v: %d services, %d metrics, %d database updates",
		duration, len(services), len(filteredMetrics), updates)

	return nil
}

// filterMetrics filters out metrics matching exclude patterns
func (ds *DiscoveryService) filterMetrics(metricNames []string) []string {
	if len(ds.excludePatterns) == 0 {
		return metricNames
	}

	filtered := make([]string, 0, len(metricNames))
	for _, metric := range metricNames {
		excluded := false
		for _, pattern := range ds.excludePatterns {
			if pattern.MatchString(metric) {
				excluded = true
				break
			}
		}
		if !excluded {
			filtered = append(filtered, metric)
		}
	}

	return filtered
}

// discoverServices discovers services from metric names
func (ds *DiscoveryService) discoverServices(ctx context.Context, metricNames []string) ([]DiscoveredService, error) {
	serviceMap := make(map[string]*DiscoveredService)

	for _, metricName := range metricNames {
		// Extract all services that have this metric
		serviceInfos := ds.extractAllServicesForMetric(ctx, metricName)

		for _, info := range serviceInfos {
			serviceName := info.Name
			namespace := info.Namespace

			if serviceName == "" || serviceName == "unknown" {
				continue
			}

			// Filter by configured namespaces if specified
			if len(ds.config.Namespaces) > 0 {
				found := false
				for _, ns := range ds.config.Namespaces {
					if ns == namespace {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}

			key := fmt.Sprintf("%s/%s", namespace, serviceName)
			if service, exists := serviceMap[key]; exists {
				service.Metrics = append(service.Metrics, metricName)
			} else {
				serviceMap[key] = &DiscoveredService{
					Name:      serviceName,
					Namespace: namespace,
					Labels: map[string]string{
						"namespace": namespace,
					},
					Metrics: []string{metricName},
				}
			}
		}
	}

	// Convert map to slice
	services := make([]DiscoveredService, 0, len(serviceMap))
	for _, service := range serviceMap {
		services = append(services, *service)
	}

	return services, nil
}

// ServiceInfo holds discovered service information
type ServiceInfo struct {
	Name      string
	Namespace string
}

// extractAllServicesForMetric extracts all services that have this metric
func (ds *DiscoveryService) extractAllServicesForMetric(ctx context.Context, metricName string) []ServiceInfo {
	var results []ServiceInfo
	serviceNames := make(map[string]bool)

	// Try to get services from label values
	for _, labelName := range ds.config.ServiceLabelNames {
		values, err := ds.client.GetLabelValues(ctx, labelName, metricName)
		if err == nil && len(values) > 0 {
			// Found services with this label - add all of them
			for _, serviceName := range values {
				if serviceName == "" || serviceNames[serviceName] {
					continue
				}
				serviceNames[serviceName] = true

				// Get namespace for this service
				namespace := "default"
				namespaceValues, err := ds.client.GetLabelValues(ctx, "namespace", metricName)
				if err == nil && len(namespaceValues) > 0 {
					namespace = namespaceValues[0]
				}

				results = append(results, ServiceInfo{
					Name:      serviceName,
					Namespace: namespace,
				})
			}
			// If we found services with this label, don't try other labels
			if len(results) > 0 {
				break
			}
		}
	}

	// If no services found from labels, try to extract from metric name
	if len(results) == 0 {
		serviceName := ds.extractServiceFromMetricName(metricName)
		if serviceName != "" && serviceName != "unknown" {
			results = append(results, ServiceInfo{
				Name:      serviceName,
				Namespace: "default",
			})
		}
	}

	return results
}

// extractServiceInfo extracts service name and namespace from a metric (legacy, kept for compatibility)
func (ds *DiscoveryService) extractServiceInfo(ctx context.Context, metricName string) (serviceName, namespace string) {
	infos := ds.extractAllServicesForMetric(ctx, metricName)
	if len(infos) > 0 {
		return infos[0].Name, infos[0].Namespace
	}
	return "", "default"
}

// extractServiceFromMetricName extracts service name from metric name using patterns
func (ds *DiscoveryService) extractServiceFromMetricName(metricName string) string {
	// Pattern 1: service_metric_name
	re1 := regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9_-]*?)_.*`)
	if matches := re1.FindStringSubmatch(metricName); len(matches) > 1 {
		service := matches[1]
		if !ds.isCommonMetricWord(service) {
			return service
		}
	}

	// Pattern 2: prefix_service_total
	re2 := regexp.MustCompile(`^.*_([a-zA-Z][a-zA-Z0-9_-]*?)_total$`)
	if matches := re2.FindStringSubmatch(metricName); len(matches) > 1 {
		service := matches[1]
		if !ds.isCommonMetricWord(service) {
			return service
		}
	}

	// Pattern 3: prefix_service_count
	re3 := regexp.MustCompile(`^.*_([a-zA-Z][a-zA-Z0-9_-]*?)_count$`)
	if matches := re3.FindStringSubmatch(metricName); len(matches) > 1 {
		service := matches[1]
		if !ds.isCommonMetricWord(service) {
			return service
		}
	}

	return "unknown"
}

// isCommonMetricWord checks if a word is a common metric term (not a service name)
func (ds *DiscoveryService) isCommonMetricWord(word string) bool {
	commonWords := []string{
		"http", "https", "tcp", "udp", "grpc",
		"cpu", "memory", "disk", "network", "io",
		"request", "requests", "response", "responses",
		"latency", "duration", "time", "rate",
		"error", "errors", "success", "failure",
		"total", "count", "sum", "avg", "max", "min",
		"bytes", "seconds", "milliseconds",
		"up", "down", "status", "health",
		"api", "db", "database", "cache", "queue",
		"go", "process", "node", "system",
		"gauge", "counter", "histogram", "summary",
	}

	wordLower := strings.ToLower(word)
	for _, common := range commonWords {
		if wordLower == common {
			return true
		}
	}
	return false
}

// updateDatabase updates the database with discovered services
func (ds *DiscoveryService) updateDatabase(ctx context.Context, services []DiscoveredService) (int, error) {
	updates := 0

	for _, discovered := range services {
		// Check if service exists
		existing, err := ds.mapper.GetServiceByName(ctx, discovered.Name, discovered.Namespace)
		if err != nil {
			// Service doesn't exist, create it
			service, err := ds.mapper.CreateService(ctx, discovered.Name, discovered.Namespace, discovered.Labels)
			if err != nil {
				log.Printf("Failed to create service %s/%s: %v", discovered.Namespace, discovered.Name, err)
				continue
			}
			log.Printf("Created new service: %s/%s with %d metrics", discovered.Namespace, discovered.Name, len(discovered.Metrics))
			updates++

			// Update metrics for new service
			if err := ds.mapper.UpdateServiceMetrics(ctx, service.ID, discovered.Metrics); err != nil {
				log.Printf("Failed to update metrics for service %s: %v", service.ID, err)
			}
		} else {
			// Service exists, check if we need to update metrics
			if err := ds.mapper.UpdateServiceMetrics(ctx, existing.ID, discovered.Metrics); err != nil {
				log.Printf("Failed to update metrics for service %s: %v", existing.ID, err)
			} else {
				updates++
			}
		}
	}

	return updates, nil
}
