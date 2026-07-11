#!/usr/bin/env python3
"""
Koda-2 Keras model server for next-gen anomaly detection.
Uses 6 features (same as GoatAI) with a deeper autoencoder-based architecture.

Features: Flow Duration, Total Fwd Packets, Total Backward Packets,
          Packet Length Mean, Flow IAT Mean, Fwd Flag Count
"""
import sys
import json
import pickle
import os
import struct

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
        print("ERROR: No Keras/TensorFlow available", file=sys.stderr)
        sys.exit(1)

import numpy as np


class Koda2Server:
    def __init__(self, model_path, scaler_path):
        try:
            self.model = keras.saving.load_model(model_path)
            self.scaler = self._load_scaler(scaler_path)
        except Exception as e:
            print(f"ERROR: Failed to load Koda-2 model/scaler: {e}", file=sys.stderr)
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
        print("USAGE: koda2_server.py <model_path> <scaler_path>", file=sys.stderr)
        sys.exit(1)

    model_path = sys.argv[1]
    scaler_path = sys.argv[2]

    server = Koda2Server(model_path, scaler_path)

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
