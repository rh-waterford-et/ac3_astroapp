#!/usr/bin/env python3
"""
Mock Prediction API

Provides a mock API endpoint that accepts job metrics and returns a predicted job time.
Used for testing the predictor client component.
"""

import logging
import random
from flask import Flask, request, jsonify

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)

app.config['BASE']=30

@app.route('/health', methods=['GET'])
def health():
    """Health check endpoint."""
    return jsonify({'status': 'healthy'}), 200

@app.route('/setbase', methods=['GET'])
def setbase():
    base = request.args.get('base', 'default base')
    app.config['BASE']=base
    logger.warning(f"Base set to: {base}")
    return jsonify({'Base=': base}), 200

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
        data = request.get_json()
        base = int(app.config['BASE'])
        
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

        # Mock prediction logic: simple formula with some randomness
        # Base time: job_size / num_processors
        # Queue penalty: queue_len * random factor
        # Add some random noise to simulate realistic predictions

        
        queue_penalty = queue_len * random.uniform(5, 15)
        noise = random.uniform(-10, 10)

        logger.info(f"base = {base}")
        
        predicted_time = max(1.0, base + queue_penalty + noise)

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
        return jsonify({'error': 'Internal server error'}), 500


if __name__ == '__main__':
    logger.info("Starting Mock Prediction API on port 5000")
    base = 30
    app.run(host='0.0.0.0', port=5000, debug=False)
