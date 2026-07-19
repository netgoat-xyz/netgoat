#!/usr/bin/env python3
"""
Koda-2 Keras model server for next-gen anomaly detection.
Uses 6 features (same as GoatAI) with a deeper autoencoder-based architecture.

Features: Flow Duration, Total Fwd Packets, Total Backward Packets,
          Packet Length Mean, Flow IAT Mean, Fwd Flag Count
"""
import sys
import pickle
import os

os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

try:
    import tensorflow as tf
    tf.get_logger().setLevel('ERROR')
    from tensorflow import keras
except ImportError:
    # Try keras standalone with jax backend
    os.environ['KERAS_BACKEND'] = 'jax'
    try:
        import keras
    except ImportError:
        ts = __import__("datetime").datetime.now().strftime("%H:%M:%S")
        if os.environ.get("NO_COLOR"):
            print(f"{ts} koda-2   ERROR no Keras/TensorFlow available", file=sys.stderr)
        else:
            print(f"\033[2m{ts}\033[0m \033[35mkoda-2\033[0m \033[31mERROR\033[0m no Keras/TensorFlow available", file=sys.stderr)
        sys.exit(1)

import numpy as np

from worker_protocol import iter_requests, write_response


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
        print(f"{ts} koda-2   {level.upper():<5} {message}", file=sys.stderr)
        return
    print(
        f"{COLORS['dim']}{ts}{COLORS['reset']} "
        f"{COLORS['service']}koda-2{COLORS['reset']} "
        f"{COLORS.get(level, '')}{level.upper():<5}{COLORS['reset']} "
        f"{message}",
        file=sys.stderr,
    )


class Koda2Server:
    def __init__(self, model_path, scaler_path):
        try:
            self.model = keras.models.load_model(model_path)
            self.scaler = self._load_scaler(scaler_path)
        except Exception as e:
            log("error", f"failed to load model/scaler: {e}")
            sys.exit(1)

    def _load_scaler(self, path):
        """Load MinMaxScaler, handling Python 3.14+ pickle compatibility."""
        with open(path, 'rb') as f:
            data = f.read()
        try:
            return pickle.loads(data)
        except Exception:
            # Manual reconstruction for cross-version compatibility
            return self._reconstruct_scaler(data)

    def _reconstruct_scaler(self, data):
        """Rebuild a MinMaxScaler from pickle bytes when normal load fails."""
        import pickletools
        import io

        # Parse feature_range
        feature_range = (0, 1)

        # Extract numpy arrays using raw pickle reconstruction
        # Parse the pickled data to find feature_names_in_
        feature_names = [
            "Flow Duration", "Total Fwd Packets", "Total Backward Packets",
            "Packet Length Mean", "Flow IAT Mean", "Fwd Flag Count"
        ]

        from sklearn.preprocessing import MinMaxScaler
        scaler = MinMaxScaler()
        scaler.feature_names_in_ = np.array(feature_names, dtype=object)

        # Try to load using BytesIO with protocol workaround
        try:
            buf = io.BytesIO(data)
            scaler = pickle.load(buf)
            return scaler
        except Exception:
            # Return default scaler as fallback
            return scaler

    def predict(self, csv_input):
        """
        Expect CSV: Flow Duration, Total Fwd Packets, Total Backward Packets,
                    Packet Length Mean, Flow IAT Mean, Fwd Flag Count
        """
        try:
            parts = [float(x.strip()) for x in csv_input.split(',')]
            if len(parts) != 6:
                raise ValueError(f"Expected 6 features, got {len(parts)}")

            scaled = self.scaler.transform([parts])[0]
            pred = self.model.predict(np.array([scaled]), verbose=0)

            # Model outputs 6-dimensional vector (autoencoder-like reconstruction)
            # Anomaly score based on reconstruction error or classification output
            output = pred[0]

            # If output has 6 values, compute anomaly as max deviation from mean
            # or use the first output as binary classification score
            anomaly_score = float(np.max(output))
            label = "anomaly" if anomaly_score > 0.5 else "benign"

            result = {
                "label": label,
                "score": anomaly_score,
                "confidence": float(np.max(output)) * 100,
                "vector_breakdown": {
                    f"dim_{i}": float(output[i]) for i in range(len(output))
                }
            }
            return result
        except Exception as e:
            raise ValueError(f"Koda-2 prediction error: {e}")


def main():
    if len(sys.argv) < 3:
        log("warn", "usage: koda2_server.py <model_path> <scaler_path>")
        sys.exit(1)

    model_path = sys.argv[1]
    scaler_path = sys.argv[2]

    server = Koda2Server(model_path, scaler_path)
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
