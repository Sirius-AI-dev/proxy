package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/config"
	"github.com/unibackend/uniproxy/internal/logger"
	"github.com/unibackend/uniproxy/internal/service"
	"net/http"
)

const ServiceName = "metrics"
const ProxyRequestDuration = "proxy_request_duration"
const ProxyHandlerDuration = "proxy_handler_duration"
const ProxyDatabaseDuration = "proxy_database_duration"
const ProxyApiCallDuration = "proxy_apicall_duration"

const ProxyBlacklistCount = "proxy_blacklist_count"

type measurer struct {
	summaryVec   map[string]*prometheus.SummaryVec
	summary      map[string]prometheus.Summary
	counterVec   map[string]*prometheus.CounterVec
	counter      map[string]prometheus.Counter
	gaugeVec     map[string]*prometheus.GaugeVec
	gauge        map[string]prometheus.Gauge
	histogram    map[string]prometheus.Histogram
	histogramVec map[string]*prometheus.HistogramVec

	runtimeMetricsCollector *runtimeMetricsCollector

	log *logger.Logger
}

const TypeSummaryVec = "summaryVec"
const TypeSummary = "summary"
const TypeCounterVec = "counterVec"
const TypeCounter = "counter"
const TypeGaugeVec = "gaugeVec"
const TypeGauge = "gauge"
const TypeHistogramVec = "histogramVec"
const TypeHistogram = "histogram"

type Measurer interface {
	service.Interface
	AddSummaryVec(name string, labels ...string)
	SummaryVec(name string) *prometheus.SummaryVec
	AddSummary(name string)
	Summary(name string) prometheus.Summary

	AddCounterVec(name string, labels ...string)
	CounterVec(name string) *prometheus.CounterVec
	AddCounter(name string)
	Counter(name string) prometheus.Counter

	AddGaugeVec(name string, labels ...string)
	GaugeVec(name string) *prometheus.GaugeVec
	AddGauge(name string)
	Gauge(name string) prometheus.Gauge

	AddHistogramVec(name string, labels ...string)
	HistogramVec(name string) *prometheus.HistogramVec
	AddHistogram(name string)
	Histogram(name string) prometheus.Histogram
}

func (m *measurer) AddSummaryVec(name string, labels ...string) {
	m.summaryVec[name] = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name: name,
	}, labels)
}

func (m *measurer) SummaryVec(name string) *prometheus.SummaryVec {
	if val, ok := m.summaryVec[name]; ok {
		return val
	}
	return nil
}

func (m *measurer) AddSummary(name string) {
	m.summary[name] = prometheus.NewSummary(prometheus.SummaryOpts{
		Name: name,
	})
}

func (m *measurer) Summary(name string) prometheus.Summary {
	if val, ok := m.summary[name]; ok {
		return val
	}
	return nil
}

func (m *measurer) AddCounterVec(name string, labels ...string) {
	m.counterVec[name] = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
	}, labels)
}

func (m *measurer) CounterVec(name string) *prometheus.CounterVec {
	if val, ok := m.counterVec[name]; ok {
		return val
	}
	return nil
}

func (m *measurer) AddCounter(name string) {
	m.counter[name] = prometheus.NewCounter(prometheus.CounterOpts{
		Name: name,
	})
}

func (m *measurer) Counter(name string) prometheus.Counter {
	if val, ok := m.counter[name]; ok {
		return val
	}
	return nil
}

func (m *measurer) AddGaugeVec(name string, labels ...string) {
	m.gaugeVec[name] = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
	}, labels)
}

func (m *measurer) GaugeVec(name string) *prometheus.GaugeVec {
	if val, ok := m.gaugeVec[name]; ok {
		return val
	}
	return nil
}

func (m *measurer) AddGauge(name string) {
	m.gauge[name] = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: name,
	})
}

func (m *measurer) Gauge(name string) prometheus.Gauge {
	if val, ok := m.gauge[name]; ok {
		return val
	}
	return nil
}

func (m *measurer) AddHistogramVec(name string, labels ...string) {
	m.histogramVec[name] = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: name,
	}, labels)
}

func (m *measurer) HistogramVec(name string) *prometheus.HistogramVec {
	if val, ok := m.histogramVec[name]; ok {
		return val
	}
	return nil
}

func (m *measurer) AddHistogram(name string) {
	m.histogram[name] = prometheus.NewHistogram(prometheus.HistogramOpts{
		Name: name,
	})
}

func (m *measurer) Histogram(name string) prometheus.Histogram {
	if val, ok := m.histogram[name]; ok {
		return val
	}
	return nil
}

func New(config config.Metrics, log *logger.Logger) Measurer {
	m := &measurer{
		summaryVec:   make(map[string]*prometheus.SummaryVec),
		summary:      make(map[string]prometheus.Summary),
		counterVec:   make(map[string]*prometheus.CounterVec),
		counter:      make(map[string]prometheus.Counter),
		gaugeVec:     make(map[string]*prometheus.GaugeVec),
		gauge:        make(map[string]prometheus.Gauge),
		histogram:    make(map[string]prometheus.Histogram),
		histogramVec: make(map[string]*prometheus.HistogramVec),

		log: log.WithField("service", ServiceName),
	}

	for metricName, metricInfo := range config.List {
		switch metricInfo.Type {
		case TypeSummaryVec:
			m.AddSummaryVec(metricName, metricInfo.Labels...)
		case TypeSummary:
			m.AddSummary(metricName)
		case TypeCounterVec:
			m.AddCounterVec(metricName, metricInfo.Labels...)
		case TypeCounter:
			m.AddCounter(metricName)
		case TypeGaugeVec:
			m.AddGaugeVec(metricName, metricInfo.Labels...)
		case TypeGauge:
			m.AddGauge(metricName)
		case TypeHistogramVec:
			m.AddHistogramVec(metricName, metricInfo.Labels...)
		case TypeHistogram:
			m.AddHistogram(metricName)
		}
	}

	// Run collect runtime metrics
	metricCollector := &runtimeMetricsCollector{
		config:   config,
		measurer: m,
	}
	go metricCollector.Run()

	return m
}

func (m *measurer) Do(task common.Task) service.Result {
	result := &service.CommonResult{}
	result.SetCode(http.StatusOK)

	taskData, err := task.MapData()
	data := taskData
	if err != nil {
		result.SetError(err)
		result.SetCode(http.StatusInternalServerError)
		return result
	}

	var measurerType, metric string
	var labels prometheus.Labels
	var value float64

	if val, ok := data["measurer"]; ok {
		measurerType = val.(string)
	}
	if val, ok := data["metric"]; ok {
		metric = val.(string)
	}
	if val, ok := data["labels"]; ok {
		labels = make(prometheus.Labels, len(val.(map[string]interface{})))
		for key, itemValue := range val.(map[string]interface{}) {
			labels[key] = itemValue.(string)
		}
	}
	if val, ok := data["value"]; ok {
		switch val.(type) {
		case float64:
			value = val.(float64)
		case int:
			value = float64(val.(int))
		}

	}

	switch measurerType {
	case TypeSummaryVec:
		if val := m.SummaryVec(metric); val != nil {
			val.With(labels).Observe(value)
		} else {
			m.log.Errorf("Metric '%s' for SummaryVec not found", metric)
		}
	case TypeSummary:
		if val := m.Summary(metric); val != nil {
			val.Observe(value)
		} else {
			m.log.Errorf("Metric '%s' for Summary not found", metric)
		}
	case TypeHistogramVec:
		if val := m.HistogramVec(metric); val != nil {
			val.With(labels).Observe(value)
		} else {
			m.log.Errorf("Metric '%s' for HistogramVec not found", metric)
		}
	case TypeHistogram:
		if val := m.Histogram(metric); val != nil {
			val.Observe(value)
		} else {
			m.log.Errorf("Metric '%s' for Histogram not found", metric)
		}
	case TypeGaugeVec:
		if val := m.GaugeVec(metric); val != nil {
			val.With(labels).Set(value)
		} else {
			m.log.Errorf("Metric '%s' for GaugeVec not found", metric)
		}
	case TypeGauge:
		if val := m.Gauge(metric); val != nil {
			val.Set(value)
		} else {
			m.log.Errorf("Metric '%s' for Gauge not found", metric)
		}
	case TypeCounterVec:
		if val := m.CounterVec(metric); val != nil {
			val.With(labels).Add(value)
		} else {
			m.log.Errorf("Metric '%s' for CounterVec not found", metric)
		}
	case TypeCounter:
		if val := m.Counter(metric); val != nil {
			val.Add(value)
		} else {
			m.log.Errorf("Metric '%s' for Counter not found", metric)
		}
	}

	return result
}
