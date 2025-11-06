#!/usr/bin/env python3
"""
Predictor Client Component

Queries a prediction API with Prometheus metrics (num_processors, job_size, queue_len)
and exports the averaged predicted_job_time back to Prometheus.
"""

import time
import logging
import requests
from collections import deque
from typing import Optional
from prometheus_client import start_http_server, Gauge
from prometheus_client.parser import text_string_to_metric_families
import os
import traceback

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)
logger.level


class PredictorClient:
    def __init__(self):
        """Initialize the predictor client with configuration from environment variables."""
        logger.info("Initializing PredictorClient from environment variables")

        # Configuration from environment variables
        self.predictor_api_url = os.getenv('PREDICTOR_API_URL', 'http://model-api:5000/predict')
        self.prometheus_url = os.getenv('PROMETHEUS_URL', 'https://thanos-querier.openshift-monitoring.svc.cluster.local:9091')
        self.query_interval = int(os.getenv('QUERY_INTERVAL_SECONDS', '10'))
        self.averaging_window = int(os.getenv('AVERAGING_WINDOW_SIZE', '5'))
        self.exporter_port = int(os.getenv('EXPORTER_PORT', '9090'))

        # Prometheus metric queries from environment variables
        self.num_processors_query = os.getenv('NUMPROCESSORS_QUERY', 'num_processors')
        self.job_size_query = os.getenv('JOBSIZE_QUERY', 'astroapp_job_size_mb')
        self.queue_len_query = os.getenv('QUEUELEN_QUERY', 'astroapp_job_queue_ahead_length')

        # Sliding window for averaging
        self.predictions_window = deque(maxlen=self.averaging_window)

        # Prometheus exporter gauge
        self.avg_predicted_time_gauge = Gauge(
            'predicted_job_time_avg',
            'Averaged predicted job time from prediction API'
        )

        logger.info(f"Initialized PredictorClient with window size: {self.averaging_window}")
        logger.info(f"Predictor API URL: {self.predictor_api_url}")
        logger.info(f"Prometheus URL: {self.prometheus_url}")
        logger.info(f"Query interval: {self.query_interval}s")
        logger.info(f"Exporter port: {self.exporter_port}")

    def query_prometheus(self, query: str) -> Optional[float]:
        """Query Prometheus and return the metric value."""
        token_path = "/var/run/secrets/kubernetes.io/serviceaccount/token"
        
        # Check if the token file exists
        if not os.path.exists(token_path):
            print(f"Error: Service account token not found at {token_path}")
            return None
    
        # Read the token from the file
        with open(token_path, "r") as f:
            token = f.read().strip()
        
        headers = {
            "Authorization": f"Bearer {token}" #sha256~Mfa-CvXbwRSAdWBwXbGkxtqDJak5hsTTVKVmirMaceE"
        }

        logger.info(f"Prometheus URL: {self.prometheus_url}/api/v1/query?query={query}")
        logger.info(f"Prometheus query: {query}")
        logger.info(f"HTTP Header: {headers}")
        
        try:
            response = requests.get(
                f"{self.prometheus_url}/api/v1/query",
                params={'query': query}, 
                headers=headers, 
                timeout=5,
                verify=False
            )
            response.raise_for_status()
            
            data = response.json()
            if data['status'] == 'success' and data['data']['result']:
                value = float(data['data']['result'][0]['value'][1])
                return value
            else:
                logger.warning(f"No data returned for query: {query}")
                return None
        except Exception as e:
            logger.error(f"Error querying Prometheus: {e}")
            traceback.print_exc()
            return None

    def get_prometheus_metrics(self) -> Optional[dict]:
        """Fetch all required metrics from Prometheus."""
        #num_processors = self.query_prometheus(self.num_processors_query)
        num_processors = 1
        logger.info(f"Job Size param: {self.job_size_query}")
        job_size = self.query_prometheus(self.job_size_query)
        queue_len = self.query_prometheus(self.queue_len_query)

        return {
            'num_processors': num_processors,
            'job_size': job_size,
            'queue_len': queue_len
        }

    def query_predictor_api(self, payload: dict) -> Optional[float]:
        """Query the predictor API with the metrics payload."""
        try:
            response = requests.post(
                self.predictor_api_url,
                json=payload,
                timeout=10
            )
            response.raise_for_status()
            data = response.json()

            predicted_time = data.get('predicted_job_time')
            if predicted_time is not None:
                return float(predicted_time)
            else:
                logger.error("Response missing 'predicted_job_time' field")
                return None
        except Exception as e:
            logger.error(f"Error querying predictor API: {e}")
            return None

    def update_average(self, new_value: float) -> float:
        """Add new prediction to window and return current average."""
        self.predictions_window.append(new_value)
        avg = sum(self.predictions_window) / len(self.predictions_window)
        return avg

    def run(self):
        """Main loop: query API periodically and export averaged predictions."""
        logger.info(f"Starting Prometheus exporter on port {self.exporter_port}")

        try:
            start_http_server(self.exporter_port)
            logger.info(f"Successfully started HTTP server on port {self.exporter_port}")
        except OSError as e:
            if e.errno == 98:  # Address already in use
                logger.error(f"Port {self.exporter_port} is already in use. Is another instance running?")
            elif e.errno == 13:  # Permission denied
                logger.error(f"Permission denied to bind to port {self.exporter_port}. Try a port > 1024.")
            else:
                logger.error(f"Failed to start HTTP server on port {self.exporter_port}: {e}")
            raise
        except Exception as e:
            logger.error(f"Unexpected error starting HTTP server: {e}")
            raise

        logger.info(f"Starting prediction loop (interval: {self.query_interval}s)")

        while True:
            try:
                # Get metrics from Prometheus
                metrics = self.get_prometheus_metrics()
                if metrics is None:
                    logger.warning("Skipping this cycle due to missing metrics")
                    time.sleep(self.query_interval)
                    continue

                logger.info(f"Queried Prometheus metrics: {metrics}")

                # Query predictor API
                logger.info(f"Calling Predictor API {metrics}")
                predicted_time = self.query_predictor_api(metrics)
                if predicted_time is None:
                    logger.warning("Skipping this cycle due to API error")
                    time.sleep(self.query_interval)
                    continue

                logger.info(f"Received prediction: {predicted_time}")

                # Update sliding window average
                avg_time = self.update_average(predicted_time)
                logger.info(f"Updated average (window size {len(self.predictions_window)}): {avg_time}")

                # Export to Prometheus
                self.avg_predicted_time_gauge.set(avg_time)

            except KeyboardInterrupt:
                logger.info("Shutting down...")
                break
            except Exception as e:
                logger.error(f"Unexpected error in main loop: {e}")

            time.sleep(self.query_interval)


if __name__ == '__main__':
    client = PredictorClient()
    client.run()
