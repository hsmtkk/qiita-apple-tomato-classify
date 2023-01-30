// Copyright (c) HashiCorp, Inc
// SPDX-License-Identifier: MPL-2.0
import { Construct } from "constructs";
import { App, TerraformStack } from "cdktf";
import * as google from '@cdktf/provider-google';
import { AssetType, TerraformAsset } from "cdktf/lib/terraform-asset";
import * as path from 'path';

const project = 'qiita-apple-tomato-classify';
const region = 'us-central1';

class MyStack extends TerraformStack {
  constructor(scope: Construct, id: string) {
    super(scope, id);

    new google.provider.GoogleProvider(this, 'google', {
      project,
      region,
    });

    const assetBucket = new google.storageBucket.StorageBucket(this, 'assetBucket', {
      location: region,
      name: `asset-bucket-${project}`,
    });

    new google.storageBucket.StorageBucket(this, 'datasetAppleBucket', {
      location: region,
      name: `dataset-apple-bucket-${project}`,
    });

    new google.storageBucket.StorageBucket(this, 'datasetTomatoBucket', {
      location: region,
      name: `dataset-tomato-bucket-${project}`,
    });

    const srcBucket = new google.storageBucket.StorageBucket(this, 'srcBucket', {
      location: region,
      name: `src-bucket-${project}`,
    });

    const dstAppleBucket = new google.storageBucket.StorageBucket(this, 'dstAppleBucket', {
      location: region,
      name: `dst-apple-bucket-${project}`,
    });

    const dstTomatoBucket = new google.storageBucket.StorageBucket(this, 'dstTomatoBucket', {
      location: region,
      name: `dst-tomato-bucket-${project}`,
    });

    const asset = new TerraformAsset(this, 'asset', {
      path: path.resolve('classify'),
      type: AssetType.ARCHIVE,
    });

    const assetObject = new google.storageBucketObject.StorageBucketObject(this, 'assetObject', {
      bucket: assetBucket.name,
      name: asset.assetHash,
      source: asset.path,
    });

    const storageAccount = new google.dataGoogleStorageProjectServiceAccount.DataGoogleStorageProjectServiceAccount(this, 'storageAccount');

    new google.projectIamMember.ProjectIamMember(this, 'storageAccountPubSub', {
      member: `serviceAccount:${storageAccount.emailAddress}`,
      project,
      role: 'roles/pubsub.publisher',
    });

    new google.cloudfunctions2Function.Cloudfunctions2Function(this, 'classify', {
      location: region,
      eventTrigger: {
        eventType: 'google.cloud.storage.object.v1.finalized',
        eventFilters: [{
          attribute: 'bucket',
          value: srcBucket.name,
        }],
      },
      buildConfig: {
        entryPoint: 'classify',
        runtime: 'go119',
        source: {
          storageSource: {
            bucket: assetBucket.name,
            object: assetObject.name,
          },
        },
      },
      name: 'classify',
      serviceConfig: {
        environmentVariables: {
          'DST_APPLE_BUCKET': dstAppleBucket.name,
          'DST_TOMATO_BUCKET': dstTomatoBucket.name,
        },
        minInstanceCount: 0,
        maxInstanceCount: 1,
      },
    });

  }
}

const app = new App();
new MyStack(app, "qiita-apple-tomato-classify");
app.synth();
