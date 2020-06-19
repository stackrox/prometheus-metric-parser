package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	monitoring "cloud.google.com/go/monitoring/apiv3"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/pkg/errors"
	"github.com/prometheus/prom2json"
	"google.golang.org/genproto/googleapis/api/label"
	gcloud_metric "google.golang.org/genproto/googleapis/api/metric"
	metricpb "google.golang.org/genproto/googleapis/api/metric"
	"google.golang.org/genproto/googleapis/api/monitoredres"
	monitoringpb "google.golang.org/genproto/googleapis/monitoring/v3"
)

type gcloud struct {
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

func gcloudConnect(projectID string) (gcloud, error) {
	ctx := context.Background()
	client, err := monitoring.NewMetricClient(ctx)
	return gcloud{
		projectID: projectID,
		client:    client,
		ctx:       ctx,
	}, err
}

func (g gcloud) close() {
	g.client.Close()
}

func (g gcloud) createGcloudMetricDescriptors(families []*prom2json.Family) {
	for _, family := range families {
		if family.Type != "SUMMARY" {
			_, err := g.createGcloudMetricDescriptor(family)
			if err != nil {
				log.Fatal(errors.Wrap(err, "error creating custom metric: "+family.Name))
			}
		}
	}
}

func (g gcloud) createGcloudMetricDescriptor(family *prom2json.Family) (*metricpb.MetricDescriptor, error) {
	metricName := strings.Title(strings.ReplaceAll(family.Name, "_", " "))

	valueType := valueTypeFromFamilyType(family.Type)

	type CommonMetric struct{ Labels map[string]string }

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
			panic("unexpected")
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

	md := &gcloud_metric.MetricDescriptor{
		Name:        metricName,
		Type:        "custom.googleapis.com/" + family.Name,
		Labels:      labels,
		MetricKind:  gcloud_metric.MetricDescriptor_GAUGE,
		ValueType:   valueType,
		Unit:        unit,
		Description: family.Help,
		DisplayName: metricName,
	}
	req := &monitoringpb.CreateMetricDescriptorRequest{
		Name:             "projects/" + g.projectID,
		MetricDescriptor: md,
	}
	m, err := g.client.CreateMetricDescriptor(g.ctx, req)
	if err != nil {
		return nil, fmt.Errorf("could not create custom metric: %v", err)
	}

	fmt.Printf("Created metric descriptor: %s\n", m.GetName())
	fmt.Printf("\twith labels: %s\n", m.GetLabels())
	return m, nil
}

// func deleteGcloudMetricDescriptor(projectID, name string) error {
// 	ctx := context.Background()
// 	c, err := monitoring.NewMetricClient(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	delReq := &monitoringpb.DeleteMetricDescriptorRequest{
// 		Name: "projects/" + projectID + "/metricDescriptors/" + "custom.googleapis.com/" + name,
// 	}
// 	err = c.DeleteMetricDescriptor(ctx, delReq)
// 	if err != nil {
// 		return fmt.Errorf("could not delete custom metric: %v", err)
// 	}

// 	return nil
// }

func (g gcloud) writeGcloudTimeSeriesValue(metric metric, optionLabels map[string]string, ts int64) error {
	theTimestamp := &timestamp.Timestamp{
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
	case gcloud_metric.MetricDescriptor_DOUBLE:
		value = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_DoubleValue{
				DoubleValue: metric.value,
			},
		}
	case gcloud_metric.MetricDescriptor_INT64:
		value = &monitoringpb.TypedValue{
			Value: &monitoringpb.TypedValue_Int64Value{
				Int64Value: int64(metric.value),
			},
		}
	default:
		panic("unexpected")
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
					// StartTime: theTimestamp,
					EndTime: theTimestamp,
				},
				Value: value,
			}},
		}},
	}
	fmt.Printf("will write metric: %s\n", "custom.googleapis.com/"+metric.family.Name)
	fmt.Printf("\twith labels: %s\n", labels)

	tries := 0
	for {
		tries++
		err := g.client.CreateTimeSeries(g.ctx, req)
		if err == nil {
			fmt.Printf("Done writing time series data.\n")
			return nil
		}
		if tries > 10 {
			return fmt.Errorf("could not write time series value, %v, giving up", err)
		}
		delay := time.Duration(10*tries) * time.Second
		fmt.Printf("could not write time series value, %v, trying in %s...\n", err, delay)
		time.Sleep(delay)
	}
}

func valueTypeFromFamilyType(familyType string) gcloud_metric.MetricDescriptor_ValueType {
	var valueType gcloud_metric.MetricDescriptor_ValueType

	switch familyType {
	case "HISTOGRAM":
		valueType = gcloud_metric.MetricDescriptor_DOUBLE
	case "COUNTER", "GAUGE":
		valueType = gcloud_metric.MetricDescriptor_INT64
	default:
		log.Fatalf("unexpected family type: %s", familyType)
	}

	return valueType
}
