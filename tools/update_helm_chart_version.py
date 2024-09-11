import sys
import os
import yaml

# Read the version passed as a command-line argument
new_version = sys.argv[1]

# Define the paths to the Chart.yaml and version files
chart_path = os.path.join(
    os.path.dirname(__file__),
    "../charts/node-group-opsy-ami-operator/Chart.yaml",
)
version_file_path = os.path.join(os.path.dirname(__file__), "../version")

try:
    # Load the Chart.yaml file
    with open(chart_path, "r") as file:
        chart = yaml.safe_load(file)

    # Update the version and appVersion in Chart.yaml
    chart["version"] = new_version
    chart["appVersion"] = new_version

    # Write the updated Chart.yaml file
    with open(chart_path, "w") as file:
        yaml.safe_dump(chart, file, default_flow_style=False)

    print(
        f"Updated Helm chart version to {new_version} and appVersion to {new_version}"
    )

    # Check if the version file exists, if not, create it
    if not os.path.exists(version_file_path):
        open(version_file_path, "w").close()

    # Write the new version to the version file
    with open(version_file_path, "w") as file:
        file.write(new_version)

    print(f"Updated version file to {new_version}")

except Exception as error:
    print(f"Failed to update Helm chart version or version file: {error}")
    sys.exit(1)
