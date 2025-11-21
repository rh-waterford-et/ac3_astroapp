#!/usr/bin/env python3
"""
Prediction API

Provides an API endpoint that accepts job metrics and returns a predicted job time
using a trained machine learning model.
"""

import logging
import pickle
import pandas as pd
import os
from flask import Flask, request, jsonify

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# Load the trained model at startup
MODEL_PATH = 'model.pkl'
try:
    with open(MODEL_PATH, 'rb') as f:
        model = pickle.load(f)
    logger.info(f"Successfully loaded model from {MODEL_PATH}")
except Exception as e:
    logger.error(f"Failed to load model: {e}")
    model = None

# Load test offset from environment (defaults to 0)
TEST_OFFSET = float(os.getenv('TEST_PREDICTION_OFFSET', '0'))
if TEST_OFFSET != 0:
    logger.warning(f"⚠️  TEST MODE ACTIVE: Adding {TEST_OFFSET} seconds to all predictions")
else:
    logger.info("Test offset: 0 (normal operation)")

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint."""
    model_status = 'loaded' if model is not None else 'not_loaded'
    return jsonify({
        'status': 'healthy',
        'model': model_status,
        'test_mode': TEST_OFFSET != 0,
        'test_offset': TEST_OFFSET
    }), 200

@app.route('/predict', methods=['POST'])
def predict():
    """
    Predict job time based on input metrics.

    Expected JSON payload:
    {
        "num_processors": <float>,
        "job_size": <float>,
        "queue_len": <float>
    }

    Returns:
    {
        "predicted_job_time": <float>
    }
    """
    try:
        # Check if model is loaded
        if model is None:
            logger.error("Model not loaded, cannot make predictions")
            return jsonify({'error': 'Model not available'}), 503
        
        data = request.get_json()
        
        if not data:
            logger.warning("Received request with no JSON data")
            return jsonify({'error': 'No JSON data provided'}), 400

        # Validate required fields
        required_fields = ['num_processors', 'job_size', 'queue_len']
        missing_fields = [field for field in required_fields if field not in data]

        if missing_fields:
            logger.warning(f"Missing required fields: {missing_fields}")
            return jsonify({
                'error': 'Missing required fields',
                'missing': missing_fields
            }), 400

        # Extract metrics
        num_processors = float(data['num_processors'])
        job_size = float(data['job_size'])
        queue_len = float(data['queue_len'])

        logger.info(f"Received prediction request: num_processors={num_processors}, "
                   f"job_size={job_size}, queue_len={queue_len}")

        # Prepare input data for model prediction
        # Model expects: processor_count, job_size_mb, queue_ahead_length
        input_data = pd.DataFrame({
            'processor_count': [num_processors],
            'job_size_mb': [job_size],
            'queue_ahead_length': [queue_len]
        })

        # Make prediction using the trained model
        prediction = model.predict(input_data)
        predicted_time = float(prediction[0])
        
        # Ensure prediction is positive
        predicted_time = max(1.0, predicted_time)
        
        # Apply test offset if configured
        original_prediction = predicted_time
        predicted_time += TEST_OFFSET
        
        if TEST_OFFSET != 0:
            logger.warning(f"⚠️  TEST MODE: Original={original_prediction:.2f}s, "
                         f"Offset=+{TEST_OFFSET}s, Final={predicted_time:.2f}s")
        else:
            logger.info(f"Model predicted job_time: {predicted_time:.2f}s")

        return jsonify({
            'predicted_job_time': round(predicted_time, 2),
            'metadata': {
                'num_processors': num_processors,
                'job_size': job_size,
                'queue_len': queue_len,
                'test_offset_applied': TEST_OFFSET
            }
        }), 200

    except ValueError as e:
        logger.error(f"Invalid numeric value in request: {e}")
        return jsonify({'error': 'Invalid numeric value in request'}), 400
    except Exception as e:
        logger.error(f"Error processing prediction request: {e}")
        return jsonify({'error': 'Internal server error'}), 500


if __name__ == '__main__':
    logger.info("Starting Prediction API on port 5000")
    if model is None:
        logger.warning("WARNING: Model not loaded, API will return errors for predictions")
    app.run(host='0.0.0.0', port=5000, debug=False)
