"""
Continuously compare provider and consumer buckets and transfer
new files in both directions without a sync_configs list.
"""
import logging
import subprocess
import time
import boto3
from botocore.exceptions import ClientError
from threading import Thread

# Configure logging
logging.basicConfig(
    level=logging.INFO, format="%(asctime)s - %(levelname)s - %(message)s"
)
logger = logging.getLogger(__name__)

# Initialize S3 client
s3 = boto3.client(
    "s3",
    aws_access_key_id="EEAAAAHuEn6LkwZJGpUAikGoJHyOrAra4yXAV1WVXpvA8XU0HAAAAAEB7vvNAAAAAAHu-80YrfBIAbZ4Ef4idjOF9BTQ",
    aws_secret_access_key="D3c2ApVsGGa7Wm1+pmfDwJddoPxhY3qSwU1U2EUUyKJRE07H3MjR3nwwaxim7mhY",
    endpoint_url="https://s3.eu-central-1.ionoscloud.com",
    region_name="de",
)

# Bucket Names
PROVIDER_BUCKET_NAME = "test-provider"
CONSUMER_BUCKET_NAME = "test-consumer"

# Script to trigger for transferring assets
TRANSFER_SCRIPT = "transfer.bash"

failure_counts = {}

def catalogue_reset(direction_key):
    global failure_counts
    logger.info(f"Running fallback command %s due to 3 consecutive Fetch Catalogue failures in direction: {direction_key}")
    try:
        process = subprocess.Popen(
            ["bash", "/app/restart.bash"],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1  # line-buffered
        )
        if process.stdout:
            for line in iter(process.stdout.readline, ''):
                logger.info("[Restart] %s", line.strip())
        process.wait()
    except FileNotFoundError:
        logger.error("Restart Script %s not found", "/app/restart.bash")
    except Exception as e:
        logger.error("Unexpected error", e)
    failure_counts[direction_key] = 0  # Reset counter

def get_s3_objects(bucket, prefix=""):
    """Retrieve list of object keys from S3 bucket with optional prefix."""
    try:
        paginator = s3.get_paginator("list_objects_v2")
        page_iterator = paginator.paginate(Bucket=bucket, Prefix=prefix)

        object_keys = []
        for page in page_iterator:
            if "Contents" in page:
                object_keys.extend([obj["Key"] for obj in page["Contents"]])

        return object_keys
    except ClientError as e:
        logger.error("Error listing objects in bucket %s with prefix %s: %s", bucket, prefix, e)
        return []

def trigger_transfer_script(asset_name, source_bucket, destination_bucket, direction_key):
    global failure_counts
    if direction_key not in failure_counts:
        failure_counts[direction_key] = 0
    """Trigger the external script with the asset name and stream output live."""
    logger.info(f"Transfering asset: {asset_name} from {source_bucket} to {destination_bucket}")

    try:
        process = subprocess.Popen(
            ["bash", f"/app/transfer.bash", asset_name, source_bucket, destination_bucket],
            stdout=subprocess.PIPE,
            stderr=subprocess.STDOUT,
            text=True,
            bufsize=1  # line-buffered
        )

        # Stream output line by line
        if process.stdout:
            for line in iter(process.stdout.readline, ''):
                logger.info(f"[Transfer] %s", line.strip())

        process.wait()

        if process.returncode == 100:  # Fetch Catalogue failure
            failure_counts[direction_key] += 1
            logger.error(f"Fetch Catalogue failure #{failure_counts[direction_key]} for asset: {asset_name} in direction: {direction_key}")
            if failure_counts[direction_key] >= 3:
                logger.info(f"Three consecutive Fetch Catalogue failures detected in direction: {direction_key}")
                catalogue_reset(direction_key)
        else:
            # Reset counter on success or other failures
            if failure_counts[direction_key] > 0:
                logger.info(f"Resetting failure count from {failure_counts[direction_key]} to 0 in direction: {direction_key}")
                failure_counts[direction_key] = 0
            if process.returncode != 0:
                logger.error(f"transfer.bash exited with return code {process.returncode} for asset: {asset_name}")
            else:
                logger.info(f"transfer.bash completed successfully for asset: {asset_name}")

    except FileNotFoundError:
        logger.error(f"transfer.bash not found")
    except Exception as e:
        logger.error(f"Unexpected error running script for {asset_name}: {e}")

def synchronize_buckets(source_bucket_name, destination_bucket_name, source_prefix, destination_prefix, direction_key, poll_interval=60):
    """
    Periodically compare the source bucket to the destination bucket and transfer new assets.
    """
    logger.info(
        f"[{direction_key}] Starting periodic synchronization from {source_bucket_name}/{source_prefix} to {destination_bucket_name}/{destination_prefix} every {poll_interval} seconds (Direction: {direction_key})"
    )
    while True:
        try:
            source_objects = get_s3_objects(source_bucket_name, prefix=source_prefix)
            destination_objects = get_s3_objects(destination_bucket_name, prefix=destination_prefix)

            if destination_prefix is "starlight/input/":
                processed_objects = get_s3_objects(destination_bucket_name, prefix="starlight/processed/")
                source_assets = set(obj[len(source_prefix):] for obj in source_objects)
                destination_assets = set(obj[len(destination_prefix):] for obj in destination_objects)
                processed_assets = set(obj[len("starlight/processed/"):] for obj in processed_objects)
                all_destination_assets = destination_assets.union(processed_assets)
                logger.info(f"[{direction_key}] All destination assets: {all_destination_assets}")
                logger.info(f"[{direction_key}] Source assets: {source_assets}")
                new_assets = source_assets - all_destination_assets
                logger.debug(f"[{direction_key}] New assets in {source_bucket_name}/{source_prefix}: {new_assets}")
            else:
                source_assets = set(obj[len(source_prefix):] for obj in source_objects)
                destination_assets = set(obj[len(destination_prefix):] for obj in destination_objects)
                logger.info(f"[{direction_key}] All destination assets: {destination_assets}")
                logger.info(f"[{direction_key}] Source assets: {source_assets}")
                new_assets = source_assets - destination_assets
                logger.debug(f"[{direction_key}] New assets in {source_bucket_name}/{source_prefix}: {new_assets}")
            if new_assets:
                logger.info(
                    f"[{direction_key}] Found {len(new_assets)} new assets in {source_bucket_name}/{source_prefix} compared to {destination_bucket_name}/{destination_prefix}: {new_assets} (Direction: {direction_key})"
                )
                for asset in new_assets:
                    full_asset_name = destination_prefix + asset
                    logger.info(f"[{direction_key}] Attempting to transfer new asset to {destination_bucket_name}/{destination_prefix}: {full_asset_name} (Direction: {direction_key})")
                    trigger_transfer_script(full_asset_name, source_bucket_name, destination_bucket_name, direction_key)
                    time.sleep(5)
            else:
                logger.info(
                    f"[{direction_key}] {source_bucket_name}/{source_prefix} and {destination_bucket_name}/{destination_prefix} are synchronized. (Direction: {direction_key})"
                )

            time.sleep(poll_interval)
        except Exception as e:
            logger.error(f"[{direction_key}] Error during synchronization (Direction: {direction_key}): {e}")
            time.sleep(poll_interval)

if __name__ == "__main__":
    thread_provider_to_consumer = Thread(
        target=synchronize_buckets,
        args=(
            PROVIDER_BUCKET_NAME,
            CONSUMER_BUCKET_NAME,
            "starlight/input/",
            "starlight/input/",
            "provider_to_consumer",
            60
        ),
    )
    thread_consumer_to_provider = Thread(
        target=synchronize_buckets,
        args=(
            CONSUMER_BUCKET_NAME,
            PROVIDER_BUCKET_NAME,
            "starlight/output/",
            "starlight/output/",
            "consumer_to_provider",
            60
        ),
    )

    thread_provider_to_consumer.daemon = True
    thread_consumer_to_provider.daemon = True

    thread_provider_to_consumer.start()
    thread_consumer_to_provider.start()

    # Keep the main thread alive
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        logger.info("Bidirectional synchronization stopped by user")