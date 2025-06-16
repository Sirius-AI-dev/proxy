package handler

import (
	"bytes"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/unibackend/uniproxy/internal/common"
	"github.com/unibackend/uniproxy/internal/service/metrics"
	"net/http"
)

func (s *HandleService) MetricsHandler(request *http.Request, w http.ResponseWriter) common.HandlerResponse {
	response := common.NewResponse()
	reg := prometheus.NewRegistry()

	var metricsList []string
	measurer := s.taskService.GetService(metrics.ServiceName).(metrics.Measurer)

	if metricsName := chi.URLParam(request, "metricName"); metricsName != "" {
		metricsList = append(metricsList, metricsName)
	} else {
		for metricName, metricInfo := range s.cfg.Metrics.List {
			switch metricInfo.Type {
			case metrics.TypeSummaryVec:
				reg.MustRegister(measurer.SummaryVec(metricName))
			case metrics.TypeSummary:
				reg.MustRegister(measurer.Summary(metricName))

			case metrics.TypeHistogramVec:
				reg.MustRegister(measurer.HistogramVec(metricName))
			case metrics.TypeHistogram:
				reg.MustRegister(measurer.Histogram(metricName))

			case metrics.TypeCounterVec:
				reg.MustRegister(measurer.CounterVec(metricName))
			case metrics.TypeCounter:
				reg.MustRegister(measurer.Counter(metricName))

			case metrics.TypeGaugeVec:
				reg.MustRegister(measurer.GaugeVec(metricName))
			case metrics.TypeGauge:
				reg.MustRegister(measurer.Gauge(metricName))
			}
		}
	}

	var body bytes.Buffer
	{
		metricFamilies, errMetr := reg.Gather()
		if errMetr != nil || len(metricFamilies) < 1 {
			response.SetStatus(http.StatusInternalServerError)
			response.SetError(fmt.Errorf("no metrics: %v", errMetr.Error()))
			return response
		}

		encoder := expfmt.NewEncoder(&body, expfmt.Negotiate(request.Header))
		for _, metricFamily := range metricFamilies {
			errEncode := encoder.Encode(metricFamily)
			if errEncode != nil {
				response.SetStatus(http.StatusInternalServerError)
				response.SetError(fmt.Errorf("can't encode metrics: %v", errEncode.Error()))
				return response
			}
		}
	}

	response.SetStatus(http.StatusOK)
	response.SetBody(body.Bytes())

	return response
}
