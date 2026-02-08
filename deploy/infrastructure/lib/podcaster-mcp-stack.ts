import * as cdk from 'aws-cdk-lib';
import * as certificatemanager from 'aws-cdk-lib/aws-certificatemanager';
import * as cloudfront from 'aws-cdk-lib/aws-cloudfront';
import * as origins from 'aws-cdk-lib/aws-cloudfront-origins';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import * as ecr from 'aws-cdk-lib/aws-ecr';
import * as events from 'aws-cdk-lib/aws-events';
import * as targets from 'aws-cdk-lib/aws-events-targets';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as route53 from 'aws-cdk-lib/aws-route53';
import * as route53Targets from 'aws-cdk-lib/aws-route53-targets';
import * as s3 from 'aws-cdk-lib/aws-s3';
import { Construct } from 'constructs';

interface PodcasterMcpStackProps extends cdk.StackProps {
  domainName: string;     // e.g. "podcasts.apresai.dev"
  parentDomain: string;   // e.g. "apresai.dev"
  stage: string;
}

export class PodcasterMcpStack extends cdk.Stack {
  constructor(scope: Construct, id: string, props: PodcasterMcpStackProps) {
    super(scope, id, props);

    const { domainName, parentDomain, stage } = props;

    // --- ECR Repository ---
    const ecrRepo = new ecr.Repository(this, 'McpServerRepo', {
      repositoryName: 'podcaster-mcp-server',
      removalPolicy: cdk.RemovalPolicy.RETAIN,
      lifecycleRules: [{
        maxImageCount: 10,
        description: 'Keep last 10 images',
      }],
    });

    // --- Reference existing resources ---
    const podcastBucket = s3.Bucket.fromBucketName(
      this, 'PodcastBucket',
      `apresai-podcasts-${cdk.Aws.ACCOUNT_ID}`,
    );

    const table = dynamodb.Table.fromTableName(
      this, 'PodcastsTable',
      `apresai-podcasts-${stage}`,
    );

    const hostedZone = route53.HostedZone.fromLookup(this, 'HostedZone', {
      domainName: parentDomain,
    });

    // --- CloudFront Access Logging Bucket ---
    const logBucket = new s3.Bucket(this, 'PodcastLogBucket', {
      bucketName: `apresai-podcast-logs-${cdk.Aws.ACCOUNT_ID}`,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      lifecycleRules: [{
        expiration: cdk.Duration.days(30),
      }],
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: true,
      objectOwnership: s3.ObjectOwnership.BUCKET_OWNER_PREFERRED,
    });

    // --- ACM Certificate for podcasts.apresai.dev ---
    const certificate = new certificatemanager.Certificate(this, 'PodcastCert', {
      domainName,
      validation: certificatemanager.CertificateValidation.fromDns(hostedZone),
    });

    // --- Audio Cache Policy (30-day TTL) ---
    const audioCachePolicy = new cloudfront.CachePolicy(this, 'AudioCachePolicy', {
      cachePolicyName: 'PodcastAudioCache',
      defaultTtl: cdk.Duration.days(30),
      maxTtl: cdk.Duration.days(365),
      minTtl: cdk.Duration.days(1),
    });

    // --- CloudFront Distribution ---
    const s3Origin = origins.S3BucketOrigin.withOriginAccessControl(podcastBucket);

    const distribution = new cloudfront.Distribution(this, 'PodcastDistribution', {
      comment: `Podcast audio CDN for ${domainName}`,
      defaultBehavior: {
        origin: s3Origin,
        viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
        allowedMethods: cloudfront.AllowedMethods.ALLOW_GET_HEAD_OPTIONS,
        cachedMethods: cloudfront.CachedMethods.CACHE_GET_HEAD_OPTIONS,
        cachePolicy: audioCachePolicy,
      },
      domainNames: [domainName],
      certificate,
      enableLogging: true,
      logBucket,
      logFilePrefix: 'cf-logs/',
    });

    // --- Route53 A Record ---
    new route53.ARecord(this, 'PodcastAliasRecord', {
      zone: hostedZone,
      recordName: domainName,
      target: route53.RecordTarget.fromAlias(
        new route53Targets.CloudFrontTarget(distribution),
      ),
    });

    // --- IAM Role for AgentCore container ---
    const agentCoreRole = new iam.Role(this, 'AgentCoreRole', {
      roleName: 'podcaster-mcp-agentcore',
      assumedBy: new iam.CompositePrincipal(
        new iam.ServicePrincipal('ecs-tasks.amazonaws.com'),
        new iam.ServicePrincipal('bedrock.amazonaws.com'),
        new iam.ServicePrincipal('bedrock-agentcore.amazonaws.com'),
      ),
    });

    // DynamoDB read/write (including GSI indexes)
    table.grantReadWriteData(agentCoreRole);
    agentCoreRole.addToPolicy(new iam.PolicyStatement({
      actions: [
        'dynamodb:Query',
        'dynamodb:Scan',
      ],
      resources: [
        table.tableArn + '/index/*',
      ],
    }));

    // S3 read/write to podcast bucket
    podcastBucket.grantReadWrite(agentCoreRole);

    // Secrets Manager read
    agentCoreRole.addToPolicy(new iam.PolicyStatement({
      actions: ['secretsmanager:GetSecretValue'],
      resources: [
        `arn:aws:secretsmanager:${cdk.Aws.REGION}:${cdk.Aws.ACCOUNT_ID}:secret:/podcaster/mcp/*`,
      ],
    }));

    // CloudWatch Logs
    agentCoreRole.addToPolicy(new iam.PolicyStatement({
      actions: [
        'logs:CreateLogGroup',
        'logs:CreateLogStream',
        'logs:PutLogEvents',
      ],
      resources: ['*'],
    }));

    // X-Ray tracing (for OTEL spans via OTLP exporter)
    agentCoreRole.addToPolicy(new iam.PolicyStatement({
      actions: [
        'xray:PutTraceSegments',
        'xray:PutTelemetryRecords',
      ],
      resources: ['*'],
    }));

    // ECR pull
    ecrRepo.grantPull(agentCoreRole);

    // --- Play Counter Lambda ---
    // Pre-built binary: run `make build-play-counter` before `make deploy-infra`
    const playCounterFn = new lambda.Function(this, 'PlayCounterFn', {
      functionName: 'podcaster-play-counter',
      runtime: lambda.Runtime.PROVIDED_AL2023,
      architecture: lambda.Architecture.ARM_64,
      handler: 'bootstrap',
      code: lambda.Code.fromAsset('../../deploy/lambda-build'),
      timeout: cdk.Duration.minutes(5),
      memorySize: 256,
      environment: {
        DYNAMODB_TABLE: `apresai-podcasts-${stage}`,
        LOG_BUCKET: logBucket.bucketName,
        LOG_PREFIX: 'cf-logs/',
      },
    });

    // Grant Lambda access to DynamoDB and log bucket
    table.grantReadWriteData(playCounterFn);
    logBucket.grantRead(playCounterFn);

    // EventBridge schedule: every 5 minutes
    new events.Rule(this, 'PlayCounterSchedule', {
      ruleName: 'podcaster-play-counter-schedule',
      schedule: events.Schedule.rate(cdk.Duration.minutes(5)),
      targets: [new targets.LambdaFunction(playCounterFn)],
    });

    // --- Outputs ---
    new cdk.CfnOutput(this, 'EcrRepoUri', {
      value: ecrRepo.repositoryUri,
      description: 'ECR repository URI for MCP server container',
    });

    new cdk.CfnOutput(this, 'DistributionDomainName', {
      value: distribution.distributionDomainName,
      description: 'CloudFront distribution domain',
    });

    new cdk.CfnOutput(this, 'PodcastUrl', {
      value: `https://${domainName}`,
      description: 'Podcast CDN URL',
    });

    new cdk.CfnOutput(this, 'AgentCoreRoleArn', {
      value: agentCoreRole.roleArn,
      description: 'IAM role ARN for AgentCore execution',
    });
  }
}
