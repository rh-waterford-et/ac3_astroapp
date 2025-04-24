import boto3
import time
import subprocess
import logging
from botocore.exceptions import ClientError
import pprint

# Configure logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

# Initialize S3 client
s3 = boto3.client(
    's3',
    aws_access_key_id='EEAAAAHuEn6LkwZJGpUAikGoJHyOrAra4yXAV1WVXpvA8XU0HAAAAAEB7vvNAAAAAAHu-80YrfBIAbZ4Ef4idjOF9BTQ',
    aws_secret_access_key='D3c2ApVsGGa7Wm1+pmfDwJddoPxhY3qSwU1U2EUUyKJRE07H3MjR3nwwaxim7mhY',
    endpoint_url='https://s3.eu-central-1.ionoscloud.com',
    region_name='de',
)

# Bucket to monitor
BUCKET_NAME = 'test-provider'

# Script to trigger when a new asset is found
TRIGGER_SCRIPT = 'transfer.bash'

def get_s3_objects(bucket):
    """Retrieve list of object keys from S3 bucket."""
    try:
        response = s3.list_objects_v2(Bucket=bucket)
        if 'Contents' not in response:
            return []
        return [obj['Key'] for obj in response['Contents']]
    except ClientError as e:
        logger.error(f"Error listing objects in bucket {bucket}: {e}")
        return []

def trigger_script(asset_name):
    """Trigger the external script with the asset name."""
    try:
        result = subprocess.run(
            ['bash', TRIGGER_SCRIPT, asset_name],
            capture_output=True,
            text=True,
            check=True
        )
        logger.info(f"Triggered {TRIGGER_SCRIPT} for {asset_name}. Output: {result.stdout}")
    except subprocess.CalledProcessError as e:
        logger.error(f"Error running {TRIGGER_SCRIPT} for {asset_name}: {e.stderr}")
    except FileNotFoundError:
        logger.error(f"Script {TRIGGER_SCRIPT} not found")

def poll_s3_bucket(bucket, poll_interval=60):
    """Poll S3 bucket for new assets and trigger script for new assets."""
    known_assets = set(get_s3_objects(bucket))
    logger.info(f"Initial assets: {known_assets}")
    
    while True:
        try:
            current_assets = set(get_s3_objects(bucket))
            new_assets = current_assets - known_assets

            if new_assets:
                logger.info(f"New assets found: {new_assets}")
                for asset in new_assets:
                    logger.info(f"Processing new asset: {asset}")
                    trigger_script(asset)
                    known_assets.add(asset)
            else:
                logger.debug("No new assets found")

            time.sleep(poll_interval)
        except KeyboardInterrupt:
            logger.info("Polling stopped by user")
            break
        except Exception as e:
            logger.error(f"Unexpected error during polling: {e}")
            time.sleep(poll_interval)

if __name__ == "__main__":
    # Poll every 10 seconds
    poll_s3_bucket(BUCKET_NAME, poll_interval=10)