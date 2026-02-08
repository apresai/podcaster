#!/usr/bin/env node
import * as cdk from 'aws-cdk-lib';
import { PodcasterMcpStack } from '../lib/podcaster-mcp-stack';

const app = new cdk.App();

new PodcasterMcpStack(app, 'PodcasterMcpStack', {
  env: {
    account: process.env.CDK_DEFAULT_ACCOUNT,
    region: 'us-east-1',
  },
  domainName: 'podcasts.apresai.dev',
  parentDomain: 'apresai.dev',
  stage: 'prod',
});
