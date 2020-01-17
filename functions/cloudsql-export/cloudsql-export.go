package function

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/functions/metadata"
	"golang.org/x/oauth2/google"
	sqladmin "google.golang.org/api/sqladmin/v1beta4"
)

// PubSubMessage is the payload of pubsub event
type PubSubMessage struct {
	Data       string `json:"data"`
	Attributes struct {
		Project  string `json:"project"`
		Instance string `json:"instance"`
		Database string `json:"database"`
		Bucket   string `json:"bucket"`
	} `json:"attributes"`
}

// Csqlexport will export cloudsql instances
func Csqlexport(ctx context.Context, m PubSubMessage) error {
	meta, err := metadata.FromContext(ctx)
	if err != nil {
		// Assume an error on the function invoker and try again.
		return fmt.Errorf("metadata.FromContext: %v", err)
	}

	// Ignore events that are too old.
	expiration := meta.Timestamp.Add(60 * time.Minute)
	if time.Now().After(expiration) {
		log.Printf("event timeout: halting retries for expired event '%q'", meta.EventID)
		return nil
	}

	projectName := m.Attributes.Project
	instanceName := m.Attributes.Instance
	dbName := m.Attributes.Database
	bucketName := m.Attributes.Bucket

	c, err := google.DefaultClient(ctx, sqladmin.CloudPlatformScope)
	if err != nil {
		log.Fatalf("ERROR creating google client - %v", err)
	}

	sqladminService, err := sqladmin.New(c)
	if err != nil {
		log.Fatalf("ERROR creating admin Service - %v", err)
		return err
	}

	dt := time.Now()
	dtSuffix := dt.Format("20060102150405")
	dtYearFolder := dt.Format("2006")
	dtMonthFolder := dt.Format("Jan")
	bucketPath := fmt.Sprintf("gs://%v/%v/%v/%v/%v/%v-%v-%v.gz", bucketName, projectName, instanceName, dtYearFolder, dtMonthFolder, instanceName, dbName, dtSuffix)
	log.Printf("projectName: %v instanceName: %v dbName: %v bucketName: %v bucketPath: %v", projectName, instanceName, dbName, bucketName, bucketPath)

	rb := &sqladmin.InstancesExportRequest{
		ExportContext: &sqladmin.ExportContext{
			Databases: []string{dbName},
			Uri:       bucketPath,
		},
	}

	resp, err := sqladminService.Instances.Export(projectName, instanceName, rb).Context(ctx).Do()
	if err != nil {
		log.Fatalf("ERROR exporting instance %v - %v", instanceName, err)
		return err
	}

	if resp.Error != nil {
		log.Fatalf("ERROR exporting instance - %#v\n", resp)
		return fmt.Errorf("Error within response: %v", resp)
	}

	log.Printf("Export operation %v - link: %v", resp.Status, resp.SelfLink)
	return nil
}
