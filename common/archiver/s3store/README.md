# Amazon S3 blobstore
## Configuration
See https://docs.aws.amazon.com/sdk-for-go/v1/developer-guide/configuring-sdk.html#specifying-credentials on how to set up authentication against s3

Enabling archival is done by using the configuration below. `Region` and `bucket URI` are required
```
archival:
  history:
    status: "enabled"
    enableRead: true
    provider:
      s3store:
        region: "us-east-1"
  visibility:
    status: "enabled"
    enableRead: true
    provider:
      s3store:
        region: "us-east-1"

domainDefaults:
  archival:
    history:
      status: "enabled"
      URI: "s3://<bucket-name>"
    visibility:
      status: "enabled"
      URI: "s3://<bucket-name>"
```

## URI schemes
Two URI schemes are supported:

- `s3://<bucket-name>[/<path>]` — plain S3 bucket. The hostname is the bucket
  name and is passed directly to the AWS SDK. This is the original scheme.
- `s3-ap://<account-id>/<access-point-name>[/<path>]` — S3
  [access point](https://docs.aws.amazon.com/AmazonS3/latest/userguide/access-points.html).
  The region is taken from the archiver's configured `region`, so the **same URI
  works in every region**. This is useful for global domains where the archival
  URI must be identical across clusters.

At request time, an `s3-ap` URI is reconstructed into an access point ARN of
the form `arn:aws:s3:<region>:<account-id>:accesspoint/<access-point-name>` and
passed as the `Bucket` parameter to S3 SDK calls. Each cluster contributes its
own region to the ARN.

Example — registering a global domain with an access point archival URI:

```
cadence --do my-domain domain register \
  --history_uri    "s3-ap://710914175400/cadence-archival/prod" \
  --visibility_uri "s3-ap://710914175400/cadence-archival/prod" \
  --history_archival_status enabled \
  --visibility_archival_status enabled \
  --gd true --cl primary,secondary --ac primary --rd 7
```

In `us-east-1` this resolves to the bucket
`arn:aws:s3:us-east-1:710914175400:accesspoint/cadence-archival`; in
`eu-west-1` the same URI resolves to
`arn:aws:s3:eu-west-1:710914175400:accesspoint/cadence-archival`.

### Access point IAM and `HeadBucket`
At domain-registration time the frontend calls `HeadBucket` against the
constructed ARN to verify the URI. `HeadBucket` on an access point is evaluated
against the **underlying bucket's** policy, not the access point policy — so
granting `s3:*` on the access point alone is not enough. The role used by the
Cadence services needs `s3:ListBucket` on the underlying bucket
(`arn:aws:s3:::<underlying-bucket-name>`) for the validation check to pass.

The ARN partition is inferred from the region prefix, so the same URI works
across `aws`, `aws-cn` (China), `aws-us-gov` (GovCloud), and `aws-iso`/
`aws-iso-b` (Secret/Top-Secret) partitions.

### Limitations
- The `s3-ap` scheme requires the archiver's `region` config to be set. Plain
  `s3://` URIs work without it (the SDK falls back to environment / instance
  metadata).
- `s3ForcePathStyle: true` is incompatible with access points. Leave it unset
  (the default) when using `s3-ap://`.
- Multi-Region Access Points (MRAP) are not currently supported. MRAP ARNs have
  no region component and require SigV4A signing, which is a separate scheme.

## Visibility query syntax
You can query the visibility store by using the `cadence workflow listarchived` command

The syntax for the query is based on SQL

Supported column names are
- WorkflowID *String*
- WorkflowTypeName *String*
- StartTime *Date*
- CloseTime *Date*
- SearchPrecision *String - Day, Hour, Minute, Second*

WorkflowID or WorkflowTypeName is required. If filtering on date use StartTime or CloseTime in combination with SearchPrecision.

Searching for a record will be done in times in the UTC timezone

SearchPrecision specifies what range you want to search for records. If you use `SearchPrecision = 'Day'`
it will search all records starting from `2020-01-21T00:00:00Z` to `2020-01-21T59:59:59Z` 

### Limitations

- The only operator supported is `=` due to how records are stored in s3.

### Example

*Searches for all records done in day 2020-01-21 with the specified workflow id*

`./cadence --do samples-domain workflow listarchived -q "StartTime = '2020-01-21T00:00:00Z' AND WorkflowID='workflow-id' AND SearchPrecision='Day'"`
## Storage in S3
Workflow runs are stored in s3 using the following structure
```
s3://<bucket-name>/<domain-id>/
	history/<workflow-id>/<run-id>
	visibility/
            workflowTypeName/<workflow-type-name>/
                startTimeout/2020-01-21T16:16:11Z/<run-id>
                closeTimeout/2020-01-21T16:16:11Z/<run-id>
            workflowID/<workflow-id>/
                startTimeout/2020-01-21T16:16:11Z/<run-id>
                closeTimeout/2020-01-21T16:16:11Z/<run-id>
```

For `s3-ap://` URIs, the path component after the access point name plays the
same role as the path under a bucket name. For example,
`s3-ap://710914175400/cadence-archival/prod` produces objects under
`prod/<domain-id>/history/...` inside the underlying bucket.

## Using localstack for local development
1. Install awscli from [here](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html)
2. Install localstack from [here](https://github.com/localstack/localstack#installing)
3. Launch localstack with `SERVICES=s3 localstack start`
4. Create a bucket using `aws --endpoint-url=http://localhost:4572 s3 mb s3://cadence-development` 
5. Configure archival and domainDefaults with the following configuration
```
archival:
  history:
    status: "enabled"
    enableRead: true
    provider:
      s3store:
        region: "us-east-1"
        endpoint: "http://127.0.0.1:4572"
        s3ForcePathStyle: true
  visibility:
    status: "enabled"
    enableRead: true
    provider:
      s3store:
        region: "us-east-1"
        endpoint: "http://127.0.0.1:4572"
        s3ForcePathStyle: true

domainDefaults:
  archival:
    history:
      status: "enabled"
      URI: "s3://cadence-development"
    visibility:
      status: "enabled"
      URI: "s3://cadence-development"
```
