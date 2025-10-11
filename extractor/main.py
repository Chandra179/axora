"""
Axora Content Extractor
Kafka consumer that extracts and validates web content
"""

from sub import KafkaClient
import config
import logging
from datetime import datetime

# Import our new modules
from url_validator import validate_url
from web_extractor import extract_content
from content_validator import validate_content_quality

# Setup logging
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)


def process_message(message):
    """
    Process incoming Kafka message with URL
    
    Pipeline:
    1. Validate URL
    2. Extract content
    3. Validate quality
    4. Print results
    """
    start_time = datetime.now()
    
    # Get URL from message
    url = message.get("url")
    if not url:
        logger.warning("Message missing 'url' field")
        return None
    
    logger.info(f"\n{'='*80}")
    logger.info(f"Processing URL: {url}")
    logger.info(f"{'='*80}")
    
    # STEP 1: Validate URL
    logger.info("STEP 1: Validating URL...")
    validation = validate_url(url)
    
    if not validation['valid']:
        logger.warning(f"✗ URL validation failed: {validation['reason']}")
        logger.info(f"Skipping URL: {url}\n")
        return None
    
    logger.info(f"✓ URL is valid: {validation['reason']}")
    logger.info(f"  Content-Type: {validation['content_type']}")
    
    # STEP 2: Extract content
    logger.info("\nSTEP 2: Extracting content...")
    extracted = extract_content(url)
    
    if not extracted:
        logger.error("✗ Content extraction failed")
        logger.info(f"Skipping URL: {url}\n")
        return None
    
    logger.info("✓ Content extracted successfully")
    
    # STEP 3: Validate quality
    logger.info("\nSTEP 3: Validating content quality...")
    quality = validate_content_quality(extracted)
    
    if not quality['passed']:
        logger.warning(f"✗ Quality validation failed (score: {quality['quality_score']:.1f}/100)")
        logger.info(f"Skipping URL: {url}\n")
        return None
    
    # SUCCESS - Print results
    logger.info("\n" + "="*80)
    logger.info("✓ PROCESSING SUCCESSFUL")
    logger.info("="*80)
    
    # Print extracted content
    print("\n" + "="*80)
    print("EXTRACTED CONTENT")
    print("="*80)
    print(f"\nURL: {extracted['url']}")
    print(f"Title: {extracted['title']}")
    print(f"Author: {extracted['author'] or 'N/A'}")
    print(f"Date: {extracted['date'] or 'N/A'}")
    print(f"Language: {extracted['language']}")
    print(f"Word Count: {extracted['word_count']}")
    print(f"Quality Score: {quality['quality_score']:.1f}/100")
    print("\n" + "-"*80)
    print("CONTENT TEXT:")
    print("-"*80)
    print(extracted['text'])
    print("\n" + "="*80 + "\n")
    
    # Calculate processing time
    duration = (datetime.now() - start_time).total_seconds()
    logger.info(f"Total processing time: {duration:.2f}s\n")
    
    return {
        "url": url,
        "extracted": extracted,
        "quality": quality,
        "processing_time": duration
    }


def main():
    """Main entry point for Kafka consumer"""
    logger.info("\n" + "="*80)
    logger.info("Starting Axora Content Extractor")
    logger.info("="*80)
    logger.info(f"Kafka URL: {config.KAFKA_URL}")
    logger.info(f"Topic: {config.KAFKA_TOPIC}")
    logger.info(f"Group ID: {config.KAFKA_GROUP_ID}")
    logger.info("="*80 + "\n")
    
    # Create Kafka client
    kafka_client = KafkaClient(config.KAFKA_URL)
    
    # Start consuming messages
    try:
        kafka_client.consume(
            topic=config.KAFKA_TOPIC,
            group_id=config.KAFKA_GROUP_ID,
            on_message=process_message
        )
    except KeyboardInterrupt:
        logger.info("\n\nShutting down gracefully...")
    except Exception as e:
        logger.error(f"Fatal error: {e}")
        raise


if __name__ == "__main__":
    main()