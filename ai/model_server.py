#!/usr/bin/env python3
"""
Local ML model server for anomaly detection.
Loads Keras model and sklearn scaler, processes CSV input, returns predictions.
"""
import sys
import json
import pickle
import os
import numpy as np

# Set TF to quiet mode
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

try:
    import tensorflow as tf
    tf.get_logger().setLevel('ERROR')
    from tensorflow import keras
except ImportError as e:
    print(f"ERROR: TensorFlow not installed: {e}", file=sys.stderr)
    sys.exit(1)


class ModelServer:
    def __init__(self, model_path, scaler_path):
        try:
            self.model = keras.models.load_model(model_path)
            with open(scaler_path, 'rb') as f:
                self.scaler = pickle.load(f)
        except Exception as e:
            print(f"ERROR: Failed to load model/scaler: {e}", file=sys.stderr)
            sys.exit(1)

    def predict(self, csv_input):
        """
        Expect CSV: Flow Duration, Total Fwd Packets, Total Backward Packets,
                    Packet Length Mean, Flow IAT Mean, Fwd Flag Count
        """
        try:
            parts = [float(x.strip()) for x in csv_input.split(',')]
            if len(parts) != 6:
                raise ValueError(f"Expected 6 features, got {len(parts)}")
            
            # Scale the input
            scaled = self.scaler.transform([parts])[0]
            
            pred = self.model.predict(np.array([scaled]), verbose=0)
            
            anomaly_score = float(pred[0][1]) if len(pred[0]) > 1 else float(pred[0][0])
            label = "anomaly" if anomaly_score > 0.5 else "benign"
            
            return {
                "label": label,
                "score": anomaly_score,
                "confidence": max(float(pred[0]))*100
            }
        except Exception as e:
            raise ValueError(f"Prediction error: {e}")


def main():
    if len(sys.argv) < 3:
        print("USAGE: model_server.py <model_path> <scaler_path>", file=sys.stderr)
        sys.exit(1)
    
    model_path = sys.argv[1]
    scaler_path = sys.argv[2]
    
    server = ModelServer(model_path, scaler_path)
    
    for line in sys.stdin:
        line = line.strip()
        if not line:
            continue
        try:
            result = server.predict(line)
            print(json.dumps(result))
        except Exception as e:
            print(json.dumps({"error": str(e)}))


if __name__ == '__main__':
    main()
