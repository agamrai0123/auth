package auth

import (
	"fmt"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog/log"
)

const metricNamespace = "auth_server"

type globalMetricCollector struct {
	reg             *prometheus.Registry
	gaugeMap        map[string]prometheus.Gauge
	counterMap      map[string]prometheus.Counter
	histogramMap    map[string]prometheus.Histogram
	gaugeVecMap     map[string]*prometheus.GaugeVec
	counterVecMap   map[string]*prometheus.CounterVec
	histogramVecMap map[string]*prometheus.HistogramVec
	lock            sync.Mutex
}

var (
	once sync.Once
	reg  *globalMetricCollector
)

func getMetricCollector() *globalMetricCollector {
	once.Do(func() {
		if reg == nil {
			reg = new(globalMetricCollector)
			reg.reg = prometheus.NewRegistry()
			reg.gaugeMap = make(map[string]prometheus.Gauge)
			reg.counterMap = make(map[string]prometheus.Counter)
			reg.histogramMap = make(map[string]prometheus.Histogram)
			reg.gaugeVecMap = make(map[string]*prometheus.GaugeVec)
			reg.counterVecMap = make(map[string]*prometheus.CounterVec)
			reg.histogramVecMap = make(map[string]*prometheus.HistogramVec)
			log.Debug().Msg("Global metric collector initialized")
		}
	})

	return reg
}

func getMetricRegistry() *prometheus.Registry {
	return getMetricCollector().reg
}

func RegisterGaugeMetric(name, help, namespace string) (prometheus.Gauge, error) {
	reg := getMetricCollector()
	reg.lock.Lock()
	defer reg.lock.Unlock()

	val, found := reg.gaugeMap[name]
	if found {
		return val, nil
	}

	v := prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      name,
		Help:      help,
		Namespace: namespace,
	})
	if err := reg.reg.Register(v); err != nil {
		return nil, fmt.Errorf("failed to register gauge metric: %w", err)
	}
	reg.gaugeMap[name] = v

	return v, nil
}

func RegisterCounterMetric(name, help, namespace string) (prometheus.Counter, error) {
	reg := getMetricCollector()
	reg.lock.Lock()
	defer reg.lock.Unlock()

	val, found := reg.counterMap[name]
	if found {
		return val, nil
	}

	v := prometheus.NewCounter(prometheus.CounterOpts{
		Name:      name,
		Help:      help,
		Namespace: namespace,
	})
	if err := reg.reg.Register(v); err != nil {
		return nil, fmt.Errorf("failed to register counter metric: %w", err)
	}
	reg.counterMap[name] = v

	return v, nil
}

func RegisterHistogramMetric(name, help, namespace string, buckets []float64) (prometheus.Histogram, error) {
	reg := getMetricCollector()
	reg.lock.Lock()
	defer reg.lock.Unlock()

	val, found := reg.histogramMap[name]
	if found {
		return val, nil
	}

	v := prometheus.NewHistogram(prometheus.HistogramOpts{
		Name:      name,
		Help:      help,
		Namespace: namespace,
		Buckets:   buckets,
	})
	if err := reg.reg.Register(v); err != nil {
		return nil, fmt.Errorf("failed to register histogram metric: %w", err)
	}
	reg.histogramMap[name] = v

	return v, nil
}

func registerGaugeVecMetric(name, help, namespace string, labels []string) (*prometheus.GaugeVec, error) {
	reg := getMetricCollector()
	reg.lock.Lock()
	defer reg.lock.Unlock()

	if val, found := reg.gaugeVecMap[name]; found {
		log.Debug().Str("metric", name).Msg("Gauge vector metric already registered, returning existing")
		return val, nil
	}

	if namespace == "" {
		namespace = metricNamespace
	}

	v := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      name,
		Help:      help,
		Namespace: namespace,
	}, labels)

	if err := reg.reg.Register(v); err != nil {
		log.Error().Err(err).Str("metric", name).Msg("Failed to register gauge vector metric")
		return nil, fmt.Errorf("failed to register gauge vec metric '%s': %w", name, err)
	}

	log.Debug().Str("metric", name).Strs("labels", labels).Msg("Gauge vector metric registered successfully")
	reg.gaugeVecMap[name] = v
	return v, nil
}

func registerCounterVecMetric(name, help, namespace string, labels []string) (*prometheus.CounterVec, error) {
	reg := getMetricCollector()
	reg.lock.Lock()
	defer reg.lock.Unlock()

	if val, found := reg.counterVecMap[name]; found {
		log.Debug().Str("metric", name).Msg("Counter vector metric already registered, returning existing")
		return val, nil
	}

	if namespace == "" {
		namespace = metricNamespace
	}

	v := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      name,
		Help:      help,
		Namespace: namespace,
	}, labels)

	if err := reg.reg.Register(v); err != nil {
		log.Error().Err(err).Str("metric", name).Msg("Failed to register counter vector metric")
		return nil, fmt.Errorf("failed to register counter vec metric '%s': %w", name, err)
	}

	log.Debug().Str("metric", name).Strs("labels", labels).Msg("Counter vector metric registered successfully")
	reg.counterVecMap[name] = v
	return v, nil
}

func registerHistogramVecMetric(name, help, namespace string, buckets []float64, labels []string) (*prometheus.HistogramVec, error) {
	reg := getMetricCollector()
	reg.lock.Lock()
	defer reg.lock.Unlock()

	if val, found := reg.histogramVecMap[name]; found {
		log.Debug().Str("metric", name).Msg("Histogram vector metric already registered, returning existing")
		return val, nil
	}

	if namespace == "" {
		namespace = metricNamespace
	}

	v := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      name,
		Help:      help,
		Namespace: namespace,
		Buckets:   buckets,
	}, labels)

	if err := reg.reg.Register(v); err != nil {
		log.Error().Err(err).Str("metric", name).Msg("Failed to register histogram vector metric")
		return nil, fmt.Errorf("failed to register histogram vec metric '%s': %w", name, err)
	}

	log.Debug().Str("metric", name).Strs("labels", labels).Msg("Histogram vector metric registered successfully")
	reg.histogramVecMap[name] = v
	return v, nil
}
