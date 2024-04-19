package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3/v2"
	"cloud.google.com/go/monitoring/apiv3/v2/monitoringpb"
	"github.com/pkg/errors"
	"github.com/prometheus/prom2json"
	"google.golang.org/genproto/googleapis/api/label"
	google_metric "google.golang.org/genproto/googleapis/api/metric"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	timestamp "google.golang.org/protobuf/types/known/timestamppb"
)

type gcpMonitoring struct {
	projectID string
	client    *monitoring.MetricClient
	ctx       context.Context
}

var commonMetricLabels = []*label.LabelDescriptor{
	{
		Key:         "Test",
		ValueType:   label.LabelDescriptor_STRING,
		Description: "The test performed. e.g. ci-scale",
	},
	{
		Key:         "ClusterFlavor",
		ValueType:   label.LabelDescriptor_STRING,
		Description: "The cluster flavor used. e.g. gke-default",
	},
}

func gcpMonitoringConnect(projectID string) (*gcpMonitoring, error) {
	ctx := context.Background()
	client, err := monitoring.NewMetricClient(ctx)
	if err != nil {
		return nil, err
	}
	return &gcpMonitoring{
		projectID: projectID,
		client:    client,
	}, nil
}

func (g *gcpMonitoring) close() {
	err := g.client.Close()
	if err != nil {
		log.Printf("Error closing GCP monitoring client: %+v", err)
	}
}

func (g *gcpMonitoring) createMetricDescriptors(families []*prom2json.Family) {
	fmt.Print("Creating metric descriptors")
	errorCount := 0
	for _, family := range families {
		if family.Type != "SUMMARY" {
			_, err := g.createMetricDescriptor(family)
			if err != nil {
				log.Println(errors.Wrap(err, "error creating custom metric: "+family.Name))
				errorCount++
			}
			fmt.Print(".")
		}
	}
	// This is a way to calculate 5% to avoid division/multiplying float numbers.
	// Divide both sides by 20 * len(families) and you'll get:
	// 0.05 < errorCount / len(families)
	if len(families) < 20*errorCount {
		log.Fatal("More than 5% of GCP requests failed. Exiting the job.")
	}
	fmt.Println("done")
}

func (g *gcpMonitoring) createMetricDescriptor(family *prom2json.Family) (*metricpb.MetricDescriptor, error) {
	metricName := strings.Title(strings.ReplaceAll(family.Name, "_", " "))

	valueType := valueTypeFromFamilyType(family.Type)

	var labels []*label.LabelDescriptor
	added := make(map[string]bool)
	labels = append(labels, commonMetricLabels...)
	for _, familyMetric := range family.Metrics {

		var metricLabels map[string]string
		switch family.Type {
		case "HISTOGRAM":
			metricLabels = familyMetric.(prom2json.Histogram).Labels
		case "COUNTER", "GAUGE":
			metricLabels = familyMetric.(prom2json.Metric).Labels
		default:
			log.Fatalf("unexpected family type: %v", family.Type)
		}

		for labelName := range metricLabels {
			if _, ok := added[labelName]; !ok {
				labels = append(labels, &label.LabelDescriptor{
					Key:         labelName,
					ValueType:   label.LabelDescriptor_STRING,
					Description: "",
				})
				added[labelName] = true
			}
		}
	}

	// Unit fudgery - Rox should/could communicate this in the metrics dump
	unit := "1"
	if strings.HasSuffix(family.Name, "_duration") {
		unit = "ms"
	}

	md := &google_metric.MetricDescriptor{
		Name:        metricName,
		Type:        "custom.googleapis.com/" + family.Name,
		Labels:      labels,
		MetricKind:  google_metric.MetricDescriptor_GAUGE,
		ValueType:   valueType,
		Unit:        unit,
		Description: family.Help,
		DisplayName: metricName,
	}
	req := &monitoringpb.CreateMetricDescriptorRequest{
		Name:             "projects/" + g.projectID,
		MetricDescriptor: md,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	return g.client.CreateMetricDescriptor(ctx, req)
}

func (g *gcpMonitoring) writeTimeSeriesValue(metric metric, optionLabels map[string]string, ts int64) error {
	timestamp := &timestamp.Timestamp{
		Seconds: ts,
	}

	labels := make(map[string]string)
	for labelKey, labelValue := range optionLabels {
		labels[labelKey] = labelValue
	}
	for labelKey, labelValue := range metric.labels {
		labels[labelKey] = labelValue
	}

	valueType := valueTypeFromFamilyType(metric.family.Type)
	var value *monitoringpb.TypedValue
	switch valueType {
	case google_metric.MetricDescriptor_DOUBLE:
		value = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_DoubleValue{
				DoubleValue: metric.value,
			},
		}
	case google_metric.MetricDescriptor_INT64:
		value = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: int64(metric.value),
			},
		}
	default:
		log.Fatalf("unexpected metric descriptor value type: %v", valueType)
	}

	req := &monitoringpb.CreateTimeSeriesRequest{
		Name: "projects/" + g.projectID,
		TimeSeries: []*monitoringpb.TimeSeries{{
			Metric: &metricpb.Metric{
				Type:   "custom.googleapis.com/" + metric.family.Name,
				Labels: labels,
			},
			Resource: &monitoredres.MonitoredResource{
				Type:   "global",
				Labels: map[string]string{},
			},
			Points: []*monitoringpb.Point{{
				Interval: &monitoringpb.TimeInterval{
					EndTime: timestamp,
				},
				Value: value,
			}},
		}},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	return g.client.CreateTimeSeries(ctx, req)
}

func valueTypeFromFamilyType(familyType string) google_metric.MetricDescriptor_ValueType {
	var valueType google_metric.MetricDescriptor_ValueType

	switch familyType {
	case "HISTOGRAM":
		valueType = google_metric.MetricDescriptor_DOUBLE
	case "COUNTER", "GAUGE":
		valueType = google_metric.MetricDescriptor_INT64
	default:
		log.Fatalf("unexpected family type: %s", familyType)
	}

	return valueType
}
