import * as cdk from 'aws-cdk-lib';
import * as certificatemanager from 'aws-cdk-lib/aws-certificatemanager';
import * as cloudfront from 'aws-cdk-lib/aws-cloudfront';
import * as origins from 'aws-cdk-lib/aws-cloudfront-origins';
import * as cognito from 'aws-cdk-lib/aws-cognito';
import * as dynamodb from 'aws-cdk-lib/aws-dynamodb';
import * as ecr from 'aws-cdk-lib/aws-ecr';
import * as events from 'aws-cdk-lib/aws-events';
import * as targets from 'aws-cdk-lib/aws-events-targets';
import * as iam from 'aws-cdk-lib/aws-iam';
import * as lambda from 'aws-cdk-lib/aws-lambda';
import * as route53 from 'aws-cdk-lib/aws-route53';
import * as route53Targets from 'aws-cdk-lib/aws-route53-targets';
import * as s3 from 'aws-cdk-lib/aws-s3';
import * as s3deploy from 'aws-cdk-lib/aws-s3-deployment';
import * as secretsmanager from 'aws-cdk-lib/aws-secretsmanager';
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

    // --- Podcaster's own S3 audio bucket ---
    const audioBucket = new s3.Bucket(this, 'AudioBucket', {
      bucketName: `podcaster-audio-${cdk.Aws.ACCOUNT_ID}`,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      removalPolicy: cdk.RemovalPolicy.RETAIN,
      cors: [{
        allowedMethods: [s3.HttpMethods.PUT],
        allowedOrigins: [`https://${domainName}`, 'http://localhost:3000'],
        allowedHeaders: ['*'],
        maxAge: 3600,
      }],
    });

    // --- Podcaster's own DynamoDB table ---
    const table = new dynamodb.Table(this, 'PodcasterTable', {
      tableName: `podcaster-${stage}`,
      partitionKey: { name: 'PK', type: dynamodb.AttributeType.STRING },
      sortKey: { name: 'SK', type: dynamodb.AttributeType.STRING },
      billingMode: dynamodb.BillingMode.PAY_PER_REQUEST,
      removalPolicy: cdk.RemovalPolicy.RETAIN,
    });
    table.addGlobalSecondaryIndex({
      indexName: 'GSI1',
      partitionKey: { name: 'GSI1PK', type: dynamodb.AttributeType.STRING },
      sortKey: { name: 'GSI1SK', type: dynamodb.AttributeType.STRING },
    });

    // --- Route53 hosted zone (lookup only — shared across projects) ---
    const hostedZone = route53.HostedZone.fromLookup(this, 'HostedZone', {
      domainName: parentDomain,
    });

    // --- CloudFront Access Logging Bucket ---
    const logBucket = new s3.Bucket(this, 'PodcastLogBucket', {
      bucketName: `podcaster-logs-${cdk.Aws.ACCOUNT_ID}`,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      lifecycleRules: [{
        expiration: cdk.Duration.days(30),
      }],
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: true,
      objectOwnership: s3.ObjectOwnership.BUCKET_OWNER_PREFERRED,
    });

    // --- ACM Certificates ---
    const certificate = new certificatemanager.Certificate(this, 'PodcastCert', {
      domainName,
      validation: certificatemanager.CertificateValidation.fromDns(hostedZone),
    });

    const authDomainName = `auth.${domainName}`;
    const authCertificate = new certificatemanager.Certificate(this, 'AuthCert', {
      domainName: authDomainName,
      validation: certificatemanager.CertificateValidation.fromDns(hostedZone),
    });

    // --- Audio Cache Policy (30-day TTL) ---
    const audioCachePolicy = new cloudfront.CachePolicy(this, 'AudioCachePolicy', {
      cachePolicyName: 'PodcasterAudioCache',
      defaultTtl: cdk.Duration.days(30),
      maxTtl: cdk.Duration.days(365),
      minTtl: cdk.Duration.days(1),
    });

    // --- Cognito User Pool (for web portal authentication) ---
    const userPool = new cognito.UserPool(this, 'PortalUserPool', {
      userPoolName: 'podcaster-portal',
      selfSignUpEnabled: true,
      signInAliases: { email: true },
      autoVerify: { email: true },
      standardAttributes: {
        email: { required: true, mutable: true },
        givenName: { required: false, mutable: true },
        familyName: { required: false, mutable: true },
      },
      passwordPolicy: {
        minLength: 8,
        requireLowercase: true,
        requireUppercase: true,
        requireDigits: true,
        requireSymbols: false,
      },
      accountRecovery: cognito.AccountRecovery.EMAIL_ONLY,
      removalPolicy: cdk.RemovalPolicy.RETAIN,
    });

    const userPoolDomain = userPool.addDomain('PortalCustomDomain', {
      customDomain: {
        domainName: authDomainName,
        certificate: authCertificate,
      },
    });

    const portalClient = userPool.addClient('PortalClient', {
      userPoolClientName: 'podcaster-web-portal',
      generateSecret: true,
      authFlows: {
        userSrp: true,
        userPassword: false,
      },
      oAuth: {
        flows: { authorizationCodeGrant: true },
        scopes: [cognito.OAuthScope.OPENID, cognito.OAuthScope.EMAIL, cognito.OAuthScope.PROFILE],
        callbackUrls: [
          `https://${domainName}/api/auth/callback/cognito`,
          'http://localhost:3000/api/auth/callback/cognito',
        ],
        logoutUrls: [
          `https://${domainName}`,
          'http://localhost:3000',
        ],
      },
    });

    // --- Portal Secrets ---
    // Secrets must exist in Secrets Manager before deploying.
    // CDK resolves the values at deploy time via CloudFormation dynamic references.
    const nextAuthSecret = secretsmanager.Secret.fromSecretNameV2(
      this, 'NextAuthSecret', '/podcaster/portal/NEXTAUTH_SECRET',
    );

    // --- Portal Static Assets Bucket ---
    const staticAssetsBucket = new s3.Bucket(this, 'PortalStaticAssets', {
      bucketName: `podcaster-portal-assets-${cdk.Aws.ACCOUNT_ID}`,
      blockPublicAccess: s3.BlockPublicAccess.BLOCK_ALL,
      removalPolicy: cdk.RemovalPolicy.DESTROY,
      autoDeleteObjects: true,
    });

    // --- Portal Server Lambda (OpenNext) ---
    const portalServerFn = new lambda.Function(this, 'PortalServerFn', {
      functionName: 'podcaster-portal-server',
      runtime: lambda.Runtime.NODEJS_22_X,
      architecture: lambda.Architecture.ARM_64,
      handler: 'index.handler',
      code: lambda.Code.fromAsset('../../portal/.open-next/server-functions/default'),
      timeout: cdk.Duration.seconds(30),
      memorySize: 1024,
      environment: {
        DYNAMODB_TABLE: table.tableName,
        AWS_REGION_CUSTOM: cdk.Aws.REGION,
        COGNITO_CLIENT_ID: portalClient.userPoolClientId,
        COGNITO_CLIENT_SECRET: portalClient.userPoolClientSecret.unsafeUnwrap(),
        COGNITO_ISSUER: `https://cognito-idp.${cdk.Aws.REGION}.amazonaws.com/${userPool.userPoolId}`,
        NEXTAUTH_URL: `https://${domainName}`,
        NEXTAUTH_SECRET: nextAuthSecret.secretValue.unsafeUnwrap(),
        AUTH_SECRET: nextAuthSecret.secretValue.unsafeUnwrap(),
        AUTH_TRUST_HOST: 'true',
      },
    });
    table.grantReadWriteData(portalServerFn);
    nextAuthSecret.grantRead(portalServerFn);

    const portalFnUrl = portalServerFn.addFunctionUrl({
      authType: lambda.FunctionUrlAuthType.NONE,
    });

    new s3deploy.BucketDeployment(this, 'DeployPortalAssets', {
      sources: [s3deploy.Source.asset('../../portal/.open-next/assets')],
      destinationBucket: staticAssetsBucket,
      cacheControl: [
        s3deploy.CacheControl.maxAge(cdk.Duration.days(365)),
        s3deploy.CacheControl.setPublic(),
        s3deploy.CacheControl.immutable(),
      ],
    });

    // --- CloudFront Distribution ---
    // Parse the Lambda Function URL domain from the full URL
    const portalOriginDomain = cdk.Fn.select(2, cdk.Fn.split('/', portalFnUrl.url));

    const portalOrigin = new origins.HttpOrigin(portalOriginDomain, {
      protocolPolicy: cloudfront.OriginProtocolPolicy.HTTPS_ONLY,
    });

    const s3AudioOrigin = origins.S3BucketOrigin.withOriginAccessControl(audioBucket);
    const s3StaticOrigin = origins.S3BucketOrigin.withOriginAccessControl(staticAssetsBucket);

    // Static assets cache policy (long TTL, immutable)
    const staticCachePolicy = new cloudfront.CachePolicy(this, 'StaticCachePolicy', {
      cachePolicyName: 'PodcasterStaticCache',
      defaultTtl: cdk.Duration.days(365),
      maxTtl: cdk.Duration.days(365),
      minTtl: cdk.Duration.days(365),
    });

    const distribution = new cloudfront.Distribution(this, 'PodcastDistribution', {
      comment: `Podcaster portal + audio CDN for ${domainName}`,
      defaultBehavior: {
        origin: portalOrigin,
        viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
        allowedMethods: cloudfront.AllowedMethods.ALLOW_ALL,
        cachePolicy: cloudfront.CachePolicy.CACHING_DISABLED,
        originRequestPolicy: cloudfront.OriginRequestPolicy.ALL_VIEWER_EXCEPT_HOST_HEADER,
      },
      additionalBehaviors: {
        '/_next/static/*': {
          origin: s3StaticOrigin,
          viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
          cachePolicy: staticCachePolicy,
        },
        '/audio/*': {
          origin: s3AudioOrigin,
          viewerProtocolPolicy: cloudfront.ViewerProtocolPolicy.REDIRECT_TO_HTTPS,
          allowedMethods: cloudfront.AllowedMethods.ALLOW_GET_HEAD_OPTIONS,
          cachedMethods: cloudfront.CachedMethods.CACHE_GET_HEAD_OPTIONS,
          cachePolicy: audioCachePolicy,
        },
      },
      domainNames: [domainName],
      certificate,
      enableLogging: true,
      logBucket,
      logFilePrefix: 'cf-logs/',
    });

    // --- Route53 A Records ---
    new route53.ARecord(this, 'PodcastAliasRecord', {
      zone: hostedZone,
      recordName: domainName,
      target: route53.RecordTarget.fromAlias(
        new route53Targets.CloudFrontTarget(distribution),
      ),
    });

    new route53.ARecord(this, 'AuthAliasRecord', {
      zone: hostedZone,
      recordName: authDomainName,
      target: route53.RecordTarget.fromAlias(
        new route53Targets.UserPoolDomainTarget(userPoolDomain),
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

    // S3 read/write to audio bucket
    audioBucket.grantReadWrite(agentCoreRole);

    // Secrets Manager read (MCP server secrets)
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

    // --- AgentCore Observability ---
    // Log + trace delivery sources/destinations already created in AWS.
    // They persist across stack updates and don't need to be in CDK.

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
        DYNAMODB_TABLE: table.tableName,
        LOG_BUCKET: logBucket.bucketName,
        LOG_PREFIX: 'cf-logs/',
      },
    });

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
      description: 'Podcaster portal + audio CDN URL',
    });

    new cdk.CfnOutput(this, 'AgentCoreRoleArn', {
      value: agentCoreRole.roleArn,
      description: 'IAM role ARN for AgentCore execution',
    });

    new cdk.CfnOutput(this, 'AudioBucketName', {
      value: audioBucket.bucketName,
      description: 'S3 bucket for podcast audio files',
    });

    new cdk.CfnOutput(this, 'DynamoDBTableName', {
      value: table.tableName,
      description: 'DynamoDB table for podcaster data',
    });

    new cdk.CfnOutput(this, 'CognitoUserPoolId', {
      value: userPool.userPoolId,
      description: 'Cognito User Pool ID for web portal',
    });

    new cdk.CfnOutput(this, 'CognitoClientId', {
      value: portalClient.userPoolClientId,
      description: 'Cognito client ID for web portal',
    });

    new cdk.CfnOutput(this, 'CognitoDomain', {
      value: `https://${authDomainName}`,
      description: 'Cognito hosted UI domain',
    });

    new cdk.CfnOutput(this, 'PortalFunctionUrl', {
      value: portalFnUrl.url,
      description: 'Portal server Lambda Function URL',
    });
  }
}
