# -buster is required to have apt available
FROM openjdk:21-slim-buster

# Install curl
RUN apt update && apt install curl -y

WORKDIR /app


COPY resources /resources

# Copy jar
COPY build/libs/dataspace-connector.jar /app

# Copy the entrypoint script
COPY entrypoint.sh /entrypoint.sh

# Make the script executable
RUN chmod +x /entrypoint.sh

# Set the entrypoint to the script
ENTRYPOINT ["/entrypoint.sh"]
