from sub import KafkaClient
import config
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)


def process_message(message):
    """
    Process incoming Kafka message
    
    Args:
        message: Deserialized message data (dict)
    """
    logger.info(f"Processing message: {message}")
    
    # Add your message processing logic here
    # Example: extract data, store to database, etc.
    
    # For now, just log the message
    print(f"Message data: {message}")


def main():
    """Main entry point for Kafka consumer"""
    logger.info("Starting Axora Extractor...")
    logger.info(f"Connecting to Kafka: {config.KAFKA_URL}")
    logger.info(f"Topic: {config.KAFKA_TOPIC}")
    logger.info(f"Group ID: {config.KAFKA_GROUP_ID}")
    
    # Create Kafka client
    kafka_client = KafkaClient(config.KAFKA_URL)
    
    # Start consuming messages
    kafka_client.consume(
        topic=config.KAFKA_TOPIC,
        group_id=config.KAFKA_GROUP_ID,
        on_message=process_message
    )


if __name__ == "__main__":
    main()