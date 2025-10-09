from kafka import KafkaConsumer
from typing import Callable
from kafka.errors import KafkaError
import json
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


class KafkaClient:
    """Kafka consumer client abstraction"""
    
    def __init__(self, bootstrap_servers: str):
        self.bootstrap_servers = bootstrap_servers
        self.consumer = None
    
    def new_client(self, topic: str, group_id: str) -> KafkaConsumer:
        """Create and initialize Kafka consumer connection"""
        try:
            self.consumer = KafkaConsumer(
                topic,
                bootstrap_servers=self.bootstrap_servers,
                group_id=group_id,
                auto_offset_reset='earliest',
                enable_auto_commit=True,
                auto_commit_interval_ms=5000,
                value_deserializer=lambda m: json.loads(m.decode('utf-8'))
            )
            logger.info(f"Connected to Kafka at {self.bootstrap_servers}")
            logger.info(f"Subscribed to topic: {topic}, group: {group_id}")
            return self.consumer
        except Exception as e:
            logger.error(f"Failed to create Kafka client: {e}")
            raise
    
    
    def consume(self, topic: str, group_id: str, on_message: Callable):
        consumer = self.new_client(topic, group_id)
        try:
            logger.info("Starting message consumption...")
            while True:
                msg_pack = consumer.poll(timeout_ms=1000)
                for tp, messages in msg_pack.items():
                    for message in messages:
                        try:
                            logger.info(f"Received message from {message.topic} partition {message.partition} offset {message.offset}")
                            on_message(message.value)
                        except Exception as e:
                            logger.error(f"Error processing message: {e}")
        except KeyboardInterrupt:
            logger.info("Shutting down consumer...")
        except KafkaError as e:
            logger.error(f"Kafka fetch error: {e}")
        finally:
            consumer.close()
            logger.info("Consumer closed")