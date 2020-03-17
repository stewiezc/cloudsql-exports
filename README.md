# cloudsql-exports

Schedule the export of databases. 

This project deploys a cloud scheduler configuration and cloud function that will export every database of every online Cloud SQL instance in the project. The exports are put into a bucket in each project and have a lifecycle meant to meet REGSCI compliance applied.

## TODO

- 

## Add a project

1. create service account using instructions below.
2. create a gitlab ci/cd variable (GCP_SERVICE_ACCOUNT_CONTENTS) with the json key for the service account. Set the scope to the environment name. 
3. add steps to the pipeline for the project

example using example-project-nonprod:
```
.example-project-nonprod:
  variables:
    PROJECT: example-project-nonprod
    # daily at 6PM America/Denver
    FREQUENCY: "0 18 * * *"
  environment:
    name: example-project-nonprod
example-project-nonprod:manual:
  extends:
    - .deploy_template
    - .example-project-nonprod
  when: manual
  except:
    refs:
      - master
example-project-nonprod:master:
  extends:
    - .deploy_template
    - .example-project-nonprod
  only:
    refs:
      - master
```

## Create sa account

```
# set your project
#export PROJECT=example-project-nonprod
gcloud config set project ${PROJECT}
gcloud iam service-accounts create cloudsql-exports --display-name=cloudsql-exports
gcloud projects add-iam-policy-binding ${PROJECT} --member serviceAccount:cloudsql-exports@${PROJECT}.iam.gserviceaccount.com \
  --role 'roles/storage.admin'
gcloud projects add-iam-policy-binding ${PROJECT} --member serviceAccount:cloudsql-exports@${PROJECT}.iam.gserviceaccount.com \
  --role 'roles/pubsub.editor'
gcloud projects add-iam-policy-binding ${PROJECT} --member serviceAccount:cloudsql-exports@${PROJECT}.iam.gserviceaccount.com \
  --role 'roles/cloudsql.viewer' 
gcloud projects add-iam-policy-binding ${PROJECT} --member serviceAccount:cloudsql-exports@${PROJECT}.iam.gserviceaccount.com \
  --role 'roles/cloudfunctions.developer'
gcloud projects add-iam-policy-binding ${PROJECT} --member serviceAccount:cloudsql-exports@${PROJECT}.iam.gserviceaccount.com \
  --role 'roles/cloudscheduler.admin'
gcloud projects add-iam-policy-binding ${PROJECT} --member serviceAccount:cloudsql-exports@${PROJECT}.iam.gserviceaccount.com \
  --role 'roles/logging.configWriter'
gcloud iam service-accounts add-iam-policy-binding ${PROJECT}@appspot.gserviceaccount.com \
  --member serviceAccount:cloudsql-exports@${PROJECT}.iam.gserviceaccount.com --role 'roles/iam.serviceAccountUser'
```

## Data Lifecycle

Meant to meet REGSCI compliance

- At 90 days move to NEARLINE storage
- At 2 years (730 days) move to COLDLINE storage
- At 5 years (1825 days) delete

## More notes on compliance

consider setting a bucket retention policy and locking it. This will prevent deletion of the data. 

Not something I would recommend outside of production

https://cloud.google.com/storage/docs/bucket-lock

## Monitoring

A logging metric `cloudsql-exports_error_count` is created. You will need to create the alert policy through the GCP console.

## How to restore

1. Start by finding the exported file using gsutil. Remember that every database is exported separetely, so don't accidentally grab the 'postgres' database when you meant to get the database named after the service. 
  * the naming standard is instancename-database-YYYYMMDDHHmmss.gz
2. Grant read access to the object for the sql instance service account to the object
3. an empty db is required. 
  * consider restoring to a new instance 
  * delete and recreate it. You may have to log in and set the db into single user mode and/or delete through psql.
4. start the import via console or gcloud cli. Make sure you use the correct user to import. 

Example:
```
INSTANCE_NAME="test-postgres"
DB_NAME="test"
DB_USER="postgres"
# path to the object to import
OBJECT_PATH="gs://example-project-nonprod-cloudsql-exports/example-project-nonprod/test-postgres/test-postgres-test-20190613220956.gz"
# get the service account address
SA_ADDRESS=$(gcloud sql instances list --filter name=$INSTANCE_NAME --format json | jq '.[].serviceAccountEmailAddress' -r)
gsutil acl ch -u $SA_ADDRESS:R $OBJECT_PATH

# if not restoring to a new instance:
#gcloud sql databases delete $DB_NAME --instance=$INSTANCE_NAME
#gcloud sql databases create $DB_NAME --instance=$INSTANCE_NAME

# start the import
gcloud sql import sql $INSTANCE_NAME $OBJECT_PATH --database=$DB_NAME --user=$DB_USER
```
