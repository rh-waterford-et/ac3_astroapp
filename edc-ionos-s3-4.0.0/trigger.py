"""
Poll an S3 bucket for new assets and trigger a script when new assets are found.
"""
import logging
import subprocess
import time
import boto3
from botocore.exceptions import ClientError

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

# Bucket to monitor
BUCKET_NAME = "test-provider"

# Script to trigger when a new asset is found
TRIGGER_SCRIPT = "transfer.bash"


def get_s3_objects(bucket):
    """Retrieve list of object keys from S3 bucket."""
    try:
        response = s3.list_objects_v2(Bucket=bucket)
        if "Contents" not in response:
            return []
        return [obj["Key"] for obj in response["Contents"]]
    except ClientError as e:
        logger.error("Error listing objects in bucket %s: %s", bucket, e)
        return []


def trigger_script(asset_name):
    """Trigger the external script with the asset name."""
    try:
        result = subprocess.run(
            ["bash", f"/app/{TRIGGER_SCRIPT}", asset_name],
            capture_output=True,
            text=True,
            check=True,
        )
        logger.info(
            "Triggered %s for %s. Output:\n %s",
            TRIGGER_SCRIPT,
            asset_name,
            result.stdout
        )
    except subprocess.CalledProcessError as e:
        logger.error("Error running %s for %s: %s", TRIGGER_SCRIPT, asset_name, e)
    except FileNotFoundError:
        logger.error("Script %s not found", TRIGGER_SCRIPT)


def poll_s3_bucket(bucket, poll_interval=20):
    """Poll S3 bucket for new assets and trigger script for new assets."""
    known_assets = set(get_s3_objects(bucket))
    logger.info("Initial assets: %s", known_assets)

    while True:
        try:
            current_assets = set(get_s3_objects(bucket))
            new_assets = current_assets - known_assets

            if new_assets:
                logger.info("New assets found: %s", new_assets)
                for asset in new_assets:
                    logger.info("Processing new asset: %s", asset)
                    trigger_script(asset)
                    known_assets.add(asset)
            else:
                logger.debug("No new assets found")

            logger.info("Waiting for new assets")
            time.sleep(poll_interval)
        except KeyboardInterrupt:
            logger.info("Polling stopped by user")
            break
        except ClientError as e:
            logger.error("Unexpected error during polling: %s", e)
            time.sleep(poll_interval)


if __name__ == "__main__":
    # Poll every 10 seconds
    poll_s3_bucket(BUCKET_NAME, poll_interval=20)
