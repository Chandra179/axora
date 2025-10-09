from sub import KafkaClient
import config
import logging
from docling.document_converter import DocumentConverter

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

converter = DocumentConverter()

def process_message(message):
    url = message.get("url")
    if not url:
        return None
    
    doc = converter.convert(url).document
    markdown_text = doc.export_to_markdown()
    
    print(f"Processed {url}, text={markdown_text}")


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