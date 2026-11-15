#!/usr/bin/env python3
"""Start the EC2 instance of the werewolf resource stack and wait until it is running.

The instance id is the EC2InstanceId output of the stack. The stack name is the first argument
or AWS_RESOURCE_CFN_STACK_NAME. The region is --region, AWS_REGION, AWS_DEFAULT_REGION or the aws profile.
The credentials come from the default boto3 chain.
"""
import argparse
import os
import sys

import boto3
from botocore.exceptions import ClientError, WaiterError

INSTANCE_OUTPUT = "EC2InstanceId"


def stack_instance_id(cfn, stack_name):
    stack = cfn.describe_stacks(StackName=stack_name)["Stacks"][0]
    for out in stack.get("Outputs", []):
        if out["OutputKey"] == INSTANCE_OUTPUT:
            return out["OutputValue"]
    raise SystemExit(f"output {INSTANCE_OUTPUT} is not found in stack {stack_name}")


def instance_state(ec2, instance_id):
    return ec2.describe_instances(InstanceIds=[instance_id])["Reservations"][0]["Instances"][0]


def main():
    parser = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    parser.add_argument("stack", nargs="?", default=os.environ.get("AWS_RESOURCE_CFN_STACK_NAME"),
                        help="stack name, default $AWS_RESOURCE_CFN_STACK_NAME")
    parser.add_argument("--region", help="aws region, default $AWS_REGION, $AWS_DEFAULT_REGION or the profile region")
    args = parser.parse_args()
    if not args.stack:
        parser.error("stack name is required: pass it or set AWS_RESOURCE_CFN_STACK_NAME")

    # boto3 reads only AWS_DEFAULT_REGION, while the aws cli and the go sdk read AWS_REGION too
    session = boto3.session.Session()
    region = args.region or os.environ.get("AWS_REGION") or session.region_name
    if not region:
        parser.error("region is required: pass --region or set AWS_REGION")
    cfn, ec2 = session.client("cloudformation", region_name=region), session.client("ec2", region_name=region)

    instance_id = stack_instance_id(cfn, args.stack)
    state = instance_state(ec2, instance_id)["State"]["Name"]
    print(f"instance {instance_id} of stack {args.stack} is {state}")

    if state in ("shutting-down", "terminated"):
        raise SystemExit(f"instance is {state}, it cannot be started")
    if state == "stopping":
        print("waiting until the instance is stopped")
        ec2.get_waiter("instance_stopped").wait(InstanceIds=[instance_id])
        state = "stopped"
    if state == "stopped":
        ec2.start_instances(InstanceIds=[instance_id])
        print("start requested")

    ec2.get_waiter("instance_running").wait(InstanceIds=[instance_id])
    # the public ip changes after a stop, so the EC2InstancePublicIp stack output may be stale
    info = instance_state(ec2, instance_id)
    print(f"instance is running, public ip: {info.get('PublicIpAddress', '-')}, "
          f"private ip: {info.get('PrivateIpAddress', '-')}")


if __name__ == "__main__":
    try:
        main()
    except (ClientError, WaiterError) as err:
        sys.exit(str(err))
