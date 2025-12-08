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
from datetime import datetime
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

        # Staleness detection configuration
        self.staleness_timeout = int(os.getenv('STALENESS_TIMEOUT_SECONDS', '300'))  # 5 minutes default
        self.last_queue_len_value = None
        self.last_queue_len_change_time = time.time()

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
        logger.info(f"Staleness timeout: {self.staleness_timeout}s")
        logger.info(f"Exporter port: {self.exporter_port}")

    def _get_prometheus_headers(self) -> Optional[dict]:
        """Get authentication headers for Prometheus queries."""
        token_path = "/var/run/secrets/kubernetes.io/serviceaccount/token"
        
        if not os.path.exists(token_path):
            logger.error(f"Service account token not found at {token_path}")
            return None
    
        with open(token_path, "r") as f:
            token = f.read().strip()
        
        return {"Authorization": f"Bearer {token}"}

    def query_prometheus(self, query: str) -> Optional[float]:
        """Query Prometheus and return the metric value with the most recent timestamp.
        
        When multiple time series are returned (e.g., metrics with different labels),
        this method sorts results by timestamp in chronological order and returns
        the value from the most recently updated metric.
        """
        headers = self._get_prometheus_headers()
        if headers is None:
            return None

        logger.info(f"Prometheus URL: {self.prometheus_url}/api/v1/query?query={query}")
        logger.info(f"Prometheus query: {query}")
        
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
                results = data['data']['result']
                logger.info(f"Total results count: {len(results)}")
                
                # Sort results by timestamp in chronological order (ascending)
                # Each result has 'value': [timestamp, value_string]
                sorted_results = sorted(results, key=lambda r: float(r['value'][0]))
                
                # Take the most recent result (last in chronologically sorted list)
                most_recent = sorted_results[-1]
                value = float(most_recent['value'][1])
                timestamp = most_recent['value'][0]
                
                logger.info(f"Query '{query}' returned value={value} at timestamp={timestamp} (most recent of {len(results)} results)")
                
                # Log timestamp range for debugging
                if len(sorted_results) > 1:
                    oldest_ts = sorted_results[0]['value'][0]
                    logger.info(f"Timestamp range: oldest={oldest_ts}, newest={timestamp}")
                
                return value
            else:
                logger.warning(f"No data returned for query: {query}")
                logger.warning(f"Response data: {data}")
                return None
        except Exception as e:
            logger.error(f"Error querying Prometheus: {e}")
            traceback.print_exc()
            return None

    def query_prometheus_range_avg(self, metric_name: str, window_samples: int = 5) -> Optional[float]:
        """Query Prometheus using range query and return average of most recent N samples.
        
        This method fetches the metric values along with their corresponding
        queue_start_time timestamps, sorts jobs by their actual queue start time
        (not Prometheus scrape time), and returns the average of the most recent N samples.
        
        Args:
            metric_name: The raw metric name (without aggregation functions)
            window_samples: Number of most recent samples to average (default: 5)
            
        Returns:
            Average of the most recent N samples, or None if no data available
        """
        headers = self._get_prometheus_headers()
        if headers is None:
            return None

        logger.info(f"Prometheus query for metric: {metric_name}")
        logger.info(f"Window samples: {window_samples}")
        
        try:
            # Query for the metric values (instant query to get current state)
            response_metric = requests.get(
                f"{self.prometheus_url}/api/v1/query",
                params={'query': metric_name},
                headers=headers,
                timeout=10,
                verify=False
            )
            response_metric.raise_for_status()
            
            # Query for queue start times (to use as real timestamps)
            response_timestamps = requests.get(
                f"{self.prometheus_url}/api/v1/query",
                params={'query': 'astroapp_job_queue_start_time_seconds'},
                headers=headers,
                timeout=10,
                verify=False
            )
            response_timestamps.raise_for_status()
            
            data_metric = response_metric.json()
            data_timestamps = response_timestamps.json()
            
            if data_metric['status'] != 'success':
                logger.warning(f"Prometheus query failed for metric: {data_metric}")
                return None
            
            if data_timestamps['status'] != 'success':
                logger.warning(f"Prometheus query failed for timestamps: {data_timestamps}")
                return None
                
            results_metric = data_metric['data']['result']
            results_timestamps = data_timestamps['data']['result']
            
            if not results_metric:
                logger.warning(f"No data returned for metric: {metric_name}")
                return None
            
            # Build a lookup table: (batch_id, job_id) -> queue_start_time
            timestamp_lookup = {}
            for series in results_timestamps:
                labels = series.get('metric', {})
                batch_id = labels.get('batch_id', '')
                job_id = labels.get('job_id', '')
                if batch_id and job_id:
                    # The value is [scrape_timestamp, queue_start_time_value]
                    queue_start_time = float(series['value'][1])
                    timestamp_lookup[(batch_id, job_id)] = queue_start_time
            
            logger.info(f"Found {len(timestamp_lookup)} jobs with queue_start_time")
            
            # Collect all samples with their real timestamps
            all_samples = []
            for series in results_metric:
                labels = series.get('metric', {})
                batch_id = labels.get('batch_id', '')
                job_id = labels.get('job_id', '')
                
                # Get the metric value
                metric_value = float(series['value'][1])
                
                # Get the real timestamp from queue_start_time
                real_timestamp = timestamp_lookup.get((batch_id, job_id))
                
                if real_timestamp is not None:
                    all_samples.append((real_timestamp, metric_value, labels))
                else:
                    logger.debug(f"No queue_start_time found for batch_id={batch_id}, job_id={job_id}")
            
            if not all_samples:
                logger.warning(f"No valid samples with timestamps found for metric: {metric_name}")
                return None
            
            # Sort by queue_start_time (ascending) to get chronological order
            all_samples.sort(key=lambda x: x[0])
            
            # Take the most recent N samples (last N in sorted order)
            recent_samples = all_samples[-window_samples:]
            
            # Calculate average
            avg_value = sum(s[1] for s in recent_samples) / len(recent_samples)
            
            # Detailed logging
            logger.info(f"Total samples with valid timestamps: {len(all_samples)}")
            logger.info(f"Using {len(recent_samples)} most recent samples for averaging")
            for i, (ts, val, labels) in enumerate(recent_samples):
                job_id = labels.get('job_id', 'unknown')
                # Convert timestamp to readable format
                readable_time = datetime.fromtimestamp(ts).strftime('%Y-%m-%d %H:%M:%S')
                logger.info(f"  Sample {i+1}: queue_start_time={readable_time} ({ts}), value={val}, job_id={job_id}")
            logger.info(f"Average of last {len(recent_samples)} samples: {avg_value}")
            
            return avg_value
            
        except Exception as e:
            logger.error(f"Error in query for {metric_name}: {e}")
            traceback.print_exc()
            return None

    def check_staleness(self, current_queue_len: float) -> bool:
        """Check if the queue_len value is stale (unchanged for too long).
        
        Updates internal tracking of value changes and returns True if the value
        is considered stale (hasn't changed for staleness_timeout seconds).
        
        Args:
            current_queue_len: The current queue length average value
            
        Returns:
            True if the value is stale (should use zeros), False otherwise
        """
        current_time = time.time()
        
        # Define a small tolerance for float comparison (values within 0.1% are considered equal)
        def values_equal(a: float, b: float) -> bool:
            if a == 0 and b == 0:
                return True
            if a == 0 or b == 0:
                return False
            return abs(a - b) / max(abs(a), abs(b)) < 0.001
        
        # Check if value has changed
        if self.last_queue_len_value is None:
            # First run - initialize tracking
            self.last_queue_len_value = current_queue_len
            self.last_queue_len_change_time = current_time
            logger.info(f"Staleness tracking initialized with queue_len={current_queue_len}")
            return False
        
        if not values_equal(current_queue_len, self.last_queue_len_value):
            # Value changed - update tracking
            logger.info(f"Queue length changed: {self.last_queue_len_value} -> {current_queue_len}")
            self.last_queue_len_value = current_queue_len
            self.last_queue_len_change_time = current_time
            return False
        
        # Value unchanged - check how long
        time_since_change = current_time - self.last_queue_len_change_time
        
        if time_since_change >= self.staleness_timeout:
            logger.warning(
                f"STALE DATA DETECTED: queue_len={current_queue_len} unchanged for "
                f"{time_since_change:.1f}s (timeout: {self.staleness_timeout}s). "
                f"Returning zeros to model."
            )
            return True
        else:
            logger.info(
                f"Queue length stable at {current_queue_len} for {time_since_change:.1f}s "
                f"(timeout: {self.staleness_timeout}s)"
            )
            return False

    def get_prometheus_metrics(self) -> Optional[dict]:
        """Fetch all required metrics from Prometheus.
        
        Uses instant query for num_processors (counting pods) and
        range queries for job_size and queue_len to properly average
        the most recent N samples sorted by their actual timestamps.
        
        If the queue_len value hasn't changed for staleness_timeout seconds,
        returns zeros for job_size and queue_len to indicate no active work.
        """
        # num_processors uses instant query (counting running pods)
        num_processors = self.query_prometheus(self.num_processors_query) or 1
        
        # For job_size and queue_len, use range queries to get properly sorted samples
        # The environment variables contain the raw metric names for range queries
        logger.info(f"Job Size metric: {self.job_size_query}")
        logger.info(f"Queue Length metric: {self.queue_len_query}")
        
        # Use range query with averaging of most recent samples
        job_size = self.query_prometheus_range_avg(
            self.job_size_query, 
            self.averaging_window
        ) or 0
        
        queue_len = self.query_prometheus_range_avg(
            self.queue_len_query, 
            self.averaging_window
        ) or 0
        
        # Check for staleness - if data hasn't changed for too long, return zeros
        if self.check_staleness(queue_len):
            logger.info("Returning zero values due to stale data")
            job_size = 0
            queue_len = 0

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
