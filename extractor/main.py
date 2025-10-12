from sub import KafkaClient
import config
import structlog, logging
from datetime import datetime
import sys

from url_validator import validate_url
from web_extractor import extract_content
from quality_scoring import is_quality_content


logging.basicConfig(
    format="%(message)s",
    stream=sys.stdout,  
    level=logging.INFO,
)

structlog.configure(
    processors=[
        structlog.stdlib.filter_by_level,
        structlog.stdlib.add_logger_name,
        structlog.stdlib.add_log_level,
        structlog.stdlib.PositionalArgumentsFormatter(),
        structlog.processors.TimeStamper(fmt="iso"),
        structlog.processors.StackInfoRenderer(),
        structlog.processors.format_exc_info,
        structlog.processors.UnicodeDecoder(),
        structlog.processors.JSONRenderer()
    ],
    context_class=dict,
    logger_factory=structlog.stdlib.LoggerFactory(),
    cache_logger_on_first_use=True,
)

logger = structlog.get_logger(__name__)


def process_message(message):
    url = message.get("url")
    if not url:
        logger.warning("message_missing_url_field")
        return None
    
    try:
        validation = validate_url(url)
        
        if not validation['valid']:
            logger.warning("url_validation_failed", 
                          url=url,
                          reason=validation['reason'])
            return None
        
        extracted = extract_content(url)
        
        if not extracted:
            logger.error("content_extraction_failed", url=url)
            return None
        
        # Quality scoring check
        if not is_quality_content(extracted, target_language="en"):
            logger.warning("content_failed_quality_check", url=url)
            return None
        
        logger.info("content_passed_quality_check", url=url)
        
        # If content passes all checks, you can process it further here
        # For example: store to database, send to another topic, etc.
        return extracted
        
    except Exception as e:
        logger.error("message_processing_error", 
                    url=url,
                    error=str(e),
                    exc_info=True)
        return None


def main():
    try:
        kafka_client = KafkaClient(config.KAFKA_URL)
        kafka_client.consume(
            topic=config.KAFKA_TOPIC,
            group_id=config.KAFKA_GROUP_ID,
            on_message=process_message
        )
    except KeyboardInterrupt:
        logger.info("shutting_down_gracefully")
        sys.exit(0)
    except Exception as e:
        logger.error("fatal_error", 
                    error=str(e),
                    exc_info=True)
        sys.exit(1)


if __name__ == "__main__":
    main()