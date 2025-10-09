
import os

# Kafka configuration
KAFKA_URL = os.getenv("KAFKA_URL", "localhost:9092")
KAFKA_TOPIC = os.getenv("KAFKA_TOPIC", "web_crawl_tasks")
KAFKA_GROUP_ID = os.getenv("KAFKA_GROUP_ID", "web_crawl_tasks_group")