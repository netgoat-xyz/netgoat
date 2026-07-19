#!/usr/bin/env python3
"""
Local ML model server for anomaly detection.
Loads Keras model and sklearn scaler, processes CSV input, returns predictions.
"""
import sys
import pickle
import os
import numpy as np

from worker_protocol import iter_requests, write_response

# Set TF to quiet mode
os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

try:
    import tensorflow as tf
    tf.get_logger().setLevel('ERROR')
    from tensorflow import keras
except ImportError as e:
    ts = __import__("datetime").datetime.now().strftime("%H:%M:%S")
    if os.environ.get("NO_COLOR"):
        print(f"{ts} model    ERROR TensorFlow not installed: {e}", file=sys.stderr)
    else:
        print(f"\033[2m{ts}\033[0m \033[35mmodel\033[0m \033[31mERROR\033[0m TensorFlow not installed: {e}", file=sys.stderr)
    sys.exit(1)


COLORS = {
    "reset": "\033[0m",
    "dim": "\033[2m",
    "info": "\033[32m",
    "warn": "\033[33m",
    "error": "\033[31m",
    "service": "\033[35m",
}


def log(level, message):
    ts = __import__("datetime").datetime.now().strftime("%H:%M:%S")
    if os.environ.get("NO_COLOR"):
        print(f"{ts} model    {level.upper():<5} {message}", file=sys.stderr)
        return
    print(
        f"{COLORS['dim']}{ts}{COLORS['reset']} "
        f"{COLORS['service']}model{COLORS['reset']} "
        f"{COLORS.get(level, '')}{level.upper():<5}{COLORS['reset']} "
        f"{message}",
        file=sys.stderr,
    )


class ModelServer:
    def __init__(self, model_path, scaler_path):
        try:
            self.model = keras.models.load_model(model_path)
            with open(scaler_path, 'rb') as f:
                self.scaler = pickle.load(f)
        except Exception as e:
            log("error", f"failed to load model/scaler: {e}")
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
        log("warn", "usage: model_server.py <model_path> <scaler_path>")
        sys.exit(1)
    
    model_path = sys.argv[1]
    scaler_path = sys.argv[2]
    
    server = ModelServer(model_path, scaler_path)
    log("info", "ready")
    
    for line, protocol_error in iter_requests():
        if protocol_error:
            write_response({"error": protocol_error})
            continue
        if not line:
            continue
        try:
            result = server.predict(line)
            write_response(result)
        except Exception as e:
            write_response({"error": str(e)})


if __name__ == '__main__':
    main()
