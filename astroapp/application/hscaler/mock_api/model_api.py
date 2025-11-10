#!/usr/bin/env python3
"""
Model Prediction API

Provides a REST API endpoint that accepts job metrics and returns a predicted job time
using a trained machine learning model.
"""

import logging
import pickle
import numpy as np
from flask import Flask, request, jsonify
import os

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# Global variable to hold the model
model = None

def load_model(model_path='model.pkl'):
    """Load the trained model from disk."""
    try:
        logger.info(f"Loading model from {model_path}")
        with open(model_path, 'rb') as f:
            loaded_model = pickle.load(f)
        logger.info("Model loaded successfully")
        return loaded_model
    except Exception as e:
        logger.error(f"Failed to load model: {e}")
        raise

# Load model at startup
try:
    model = load_model()
except Exception as e:
    logger.error(f"CRITICAL: Failed to load model at startup: {e}")
    model = None

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint."""
    if model is None:
        return jsonify({
            'status': 'unhealthy',
            'reason': 'Model not loaded'
        }), 503
    return jsonify({'status': 'healthy'}), 200

@app.route('/predict', methods=['POST'])
def predict():
    """
    Predict job time based on input metrics using the trained model.

    Expected JSON payload:
    {
        "num_processors": <float>,
        "job_size": <float>,
        "queue_len": <float>
    }

    Returns:
    {
        "predicted_job_time": <float>,
        "metadata": {
            "num_processors": <float>,
            "job_size": <float>,
            "queue_len": <float>
        }
    }
    """
    try:
        if model is None:
            logger.error("Model is not loaded")
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

        # Prepare input for the model
        # The model expects features in a specific order
        # Based on the notebook, features should be: [num_processors, job_size, queue_len]
        features = np.array([[num_processors, job_size, queue_len]])
        
        # Make prediction
        prediction = model.predict(features)
        predicted_time = float(prediction[0])
        
        # Ensure predicted time is positive
        predicted_time = max(0.0, predicted_time)

        logger.info(f"Returning predicted_job_time: {predicted_time:.2f}")

        return jsonify({
            'predicted_job_time': round(predicted_time, 2),
            'metadata': {
                'num_processors': num_processors,
                'job_size': job_size,
                'queue_len': queue_len
            }
        }), 200

    except ValueError as e:
        logger.error(f"Invalid numeric value in request: {e}")
        return jsonify({'error': 'Invalid numeric value in request'}), 400
    except Exception as e:
        logger.error(f"Error processing prediction request: {e}")
        return jsonify({'error': f'Internal server error: {str(e)}'}), 500


if __name__ == '__main__':
    logger.info("Starting Model Prediction API on port 5000")
    if model is None:
        logger.warning("WARNING: Model is not loaded. API will return 503 errors.")
    app.run(host='0.0.0.0', port=5000, debug=False)


