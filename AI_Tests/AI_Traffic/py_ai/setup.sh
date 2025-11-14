#!/bin/bash

# --- Setup Script for AI Endpoint ---

VENV_NAME=".venv"
MODEL_TRAINER="network_traffic_model.py"
API_ENDPOINT="ai_endpoint.py"
REQUIREMENTS="requirements.txt"

echo "Starting setup for the Python AI Endpoint..."
echo "-------------------------------------------"

# 1. Create and Activate Virtual Environment
if [ ! -d "$VENV_NAME" ]; then
    echo "Creating virtual environment: $VENV_NAME"
    # Use python3 alias if available
    python3 -m venv "$VENV_NAME" || python -m venv "$VENV_NAME"
fi

echo "Activating virtual environment..."
# Check for Linux/Mac activation path first, then Windows
if [ -f "$VENV_NAME/bin/activate" ]; then
    source "$VENV_NAME/bin/activate"
elif [ -f "$VENV_NAME/Scripts/activate" ]; then
    source "$VENV_NAME/Scripts/activate"
else
    echo "ERROR: Could not find virtual environment activation script."
    exit 1
fi

# 2. Install Dependencies
echo "Installing dependencies from $REQUIREMENTS. This may take a moment..."
pip install --upgrade pip
if pip install -r "$REQUIREMENTS"; then
    echo "Dependencies installed successfully."
else
    echo "ERROR: Failed to install dependencies. Check your virtual environment activation and network connection."
    deactivate # Exit venv on failure
    exit 1
fi

echo "-------------------------------------------"

# 3. Train Model and Generate Assets (H5 and JSON)
echo "Running the model training script ($MODEL_TRAINER) to generate assets..."
echo "This step trains the neural network and saves 'network_traffic_model.h5' and 'scaler_params.json'."
if python "$MODEL_TRAINER"; then
    echo "Model training and asset generation complete."
else
    echo "ERROR: Model training failed. The API server cannot run without the model assets."
    deactivate
    exit 1
fi

echo "-------------------------------------------"

# 4. Start the Flask AI Endpoint
echo "Starting the Flask AI Endpoint on http://127.0.0.1:5000/..."
echo "Press Ctrl+C to stop the server."

# Execute the Flask application
python "$API_ENDPOINT"
# The script execution pauses here until the user stops the server

# Clean up (optional)
# deactivate