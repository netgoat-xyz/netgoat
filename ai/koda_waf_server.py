#!/usr/bin/env python3
"""
Koda-Waf XGBoost model server for ML-enhanced WAF attack detection.
Model: XGBClassifier with 21 engineered features extracted from raw requests.

When input is a 21-value CSV (comma-separated floats), it's used directly.
When input is a JSON object, features are extracted from the request data.

Expected CSV features (21):
  path_len, path_entropy, path_spec_chars, path_digit_ratio,
  path_cnt_sql, path_cnt_xss, path_cnt_file,
  body_len, body_entropy, body_spec_chars, body_digit_ratio,
  body_cnt_sql, body_cnt_xss, body_cnt_file,
  ua_entropy, ua_spec_chars, ua_digit_ratio,
  ua_cnt_sql, ua_cnt_xss, ua_cnt_file,
  is_post
"""
import sys
import json
import math
import re
import pickle
import os

os.environ['TF_CPP_MIN_LOG_LEVEL'] = '3'

import numpy as np


class KodaWafServer:
    def __init__(self, model_path, features_path):
        try:
            # Use standard pickle for XGBoost
            with open(model_path, 'rb') as f:
                self.model = pickle.load(f)
            with open(features_path, 'rb') as f:
                self.expected_cols = pickle.load(f)
        except Exception as e:
            print(f"ERROR: Failed to load Koda-Waf model: {e}", file=sys.stderr)
            sys.exit(1)

    def _cnt(self, text, patterns):
        count = 0
        for p in patterns:
            count += len(re.findall(p, text, re.IGNORECASE))
        return count

    def _entropy(self, s):
        if not s:
            return 0.0
        s_len = len(s)
        freqs = {}
        for c in s:
            freqs[c] = freqs.get(c, 0) + 1
        return -sum((f / s_len) * math.log2(f / s_len) for f in freqs.values())

    def extract_features(self, req):
        path = req.get("path", "")
        body = req.get("body", "")
        ua = ""
        headers = req.get("headers", {})
        if isinstance(headers, dict):
            for k, v in headers.items():
                if k.lower() == "user-agent":
                    ua = v if isinstance(v, str) else (v[0] if isinstance(v, list) else str(v))
                    break

        sql_patterns = [r'\bunion\b', r'\bselect\b', r'\bdrop\b', r'1=1', r"'--", r'\binsert\b', r'\bor\b']
        xss_patterns = [r'<script', r'javascript:', r'onerror=', r'onload=', r'<img', r'<svg', r'alert\(']
        file_patterns = [r'\.\./', r'\.\.\\', r'/etc/', r'/proc/', r'passwd', r'boot.ini', r'win.ini']

        features = {
            "path_len": len(path),
            "path_entropy": self._entropy(path),
            "path_spec_chars": sum(1 for c in path if c in "<>'\"%;=()&|`$!#*/\\"),
            "path_digit_ratio": sum(1 for c in path if c.isdigit()) / max(len(path), 1),
            "path_cnt_sql": self._cnt(path, sql_patterns),
            "path_cnt_xss": self._cnt(path, xss_patterns),
            "path_cnt_file": self._cnt(path, file_patterns),
            "body_len": len(body),
            "body_entropy": self._entropy(body),
            "body_spec_chars": sum(1 for c in body if c in "<>'\"%;=()&|`$!#*/\\"),
            "body_digit_ratio": sum(1 for c in body if c.isdigit()) / max(len(body), 1),
            "body_cnt_sql": self._cnt(body, sql_patterns),
            "body_cnt_xss": self._cnt(body, xss_patterns),
            "body_cnt_file": self._cnt(body, file_patterns),
            "ua_entropy": self._entropy(ua),
            "ua_spec_chars": sum(1 for c in ua if c in "<>'\"%;=()&|`$!#*/\\"),
            "ua_digit_ratio": sum(1 for c in ua if c.isdigit()) / max(len(ua), 1),
            "ua_cnt_sql": self._cnt(ua, sql_patterns),
            "ua_cnt_xss": self._cnt(ua, xss_patterns),
            "ua_cnt_file": self._cnt(ua, file_patterns),
            "is_post": 1.0 if req.get("method", "").upper() == "POST" else 0.0,
        }
        return features

    def predict(self, raw_input):
        try:
            # Try parsing as JSON first (raw request data)
            try:
                req = json.loads(raw_input)
                if isinstance(req, dict) and "path" in req:
                    features = self.extract_features(req)
                    df_features = features
                else:
                    raise ValueError("JSON input must contain 'path' field")
            except (json.JSONDecodeError, ValueError):
                # Fall back to CSV (21 pre-extracted features)
                parts = [float(x.strip()) for x in raw_input.split(',')]
                if len(parts) != 21:
                    raise ValueError(f"Expected 21 features, got {len(parts)}")
                feature_names = [
                    "path_len", "path_entropy", "path_spec_chars", "path_digit_ratio",
                    "path_cnt_sql", "path_cnt_xss", "path_cnt_file",
                    "body_len", "body_entropy", "body_spec_chars", "body_digit_ratio",
                    "body_cnt_sql", "body_cnt_xss", "body_cnt_file",
                    "ua_entropy", "ua_spec_chars", "ua_digit_ratio",
                    "ua_cnt_sql", "ua_cnt_xss", "ua_cnt_file",
                    "is_post"
                ]
                df_features = dict(zip(feature_names, parts))

            # Build DataFrame row matching expected columns
            row = []
            for col in self.expected_cols:
                row.append(df_features.get(col, 0.0))
            X = np.array([row])

            proba = self.model.predict_proba(X)[0][1]
            pred_class = self.model.predict(X)[0]

            label = "attack" if pred_class == 1 else "benign"

            return {
                "label": label,
                "score": float(proba),
                "confidence": float(proba * 100),
                "attack_type": label if label != "benign" else ""
            }
        except Exception as e:
            raise ValueError(f"Koda-Waf prediction error: {e}")


def main():
    if len(sys.argv) < 3:
        print("USAGE: koda_waf_server.py <model_path> <features_path>", file=sys.stderr)
        sys.exit(1)

    model_path = sys.argv[1]
    features_path = sys.argv[2]

    server = KodaWafServer(model_path, features_path)

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
