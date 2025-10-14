from sub import KafkaClient
import config
import structlog, logging
import sys
from transformers import pipeline

## DO NOT REMOVE TORCH!!!
import torch
import time
from datetime import datetime

from url_validator import validate_url
from web_extractor import extract_content
from quality_scoring import calculate_quality_score


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
pipe = pipeline(model="typeform/distilbert-base-uncased-mnli")


class RateLimitedConsumer:
    def __init__(self, messages_per_minute=15):
        self.messages_per_minute = messages_per_minute
        self.message_count = 0
        self.window_start = datetime.now()
        self.interval = 60 / messages_per_minute  # seconds between messages
        
    def should_process(self):
        now = datetime.now()
        elapsed = (now - self.window_start).total_seconds()
        
        if elapsed >= 60:
            self.message_count = 0
            self.window_start = now
            return True
        
        if self.message_count >= self.messages_per_minute:
            sleep_time = 60 - elapsed
            logger.info("rate_limit_reached", 
                       messages_processed=self.message_count,
                       sleeping_seconds=sleep_time)
            time.sleep(sleep_time)
            self.message_count = 0
            self.window_start = datetime.now()
        
        return True
    
    def wait_for_next_slot(self):
        """Wait for the appropriate interval between messages"""
        time.sleep(self.interval)


def process_message(message, rate_limiter):
    if not rate_limiter.should_process():
        return None
    
    url = message.get("url")
    if not url:
        logger.warning("message_missing_url_field")
        rate_limiter.message_count += 1
        return None
    
    try:
        validation = validate_url(url)
        
        if not validation['valid']:
            logger.warning("url_validation_failed", 
                          url=url,
                          reason=validation['reason'])
            rate_limiter.message_count += 1
            return None
        
        extracted = extract_content(url)
        
        if not extracted:
            logger.error("content_extraction_failed", url=url)
            rate_limiter.message_count += 1
            return None
        
        calc_res = calculate_quality_score(extracted, target_language="en")
        logger.info("====================================================")
        
        result = pipe(extracted.get("title") or "" + extracted.get("excerpt") or "" + extracted.get("text") or "",
            candidate_labels = [
                "economy", "macroeconomics", "microeconomics", "finance", "financial markets",
                "inflation", "deflation", "debt", "interest rates", "monetary policy", "fiscal policy",
                "bank", "treasury", "gdp", "stock market", "investment", "bonds", "currency exchange",
                "cryptocurrency", "real estate", "capital markets", "government spending", "tax policy",
                "recession", "economic growth", "public debt", "employment", "unemployment",
                "labor market", "income inequality", "global trade", "supply chain", "foreign exchange"
            ],
        )
        logger.info(extracted | calc_res | result)
        
        rate_limiter.message_count += 1
        rate_limiter.wait_for_next_slot()
        
        return extracted
        
    except Exception as e:
        logger.error("message_processing_error", 
                    url=url,
                    error=str(e),
                    exc_info=True)
        rate_limiter.message_count += 1
        return None


def main():
    # Get rate limit from environment variable, default to 10 messages per minute
    messages_per_minute = int(config.MESSAGES_PER_MINUTE if hasattr(config, 'MESSAGES_PER_MINUTE') else 10)
    
    logger.info("starting_consumer", messages_per_minute=messages_per_minute)
    rate_limiter = RateLimitedConsumer(messages_per_minute=messages_per_minute)
    
    try:
        kafka_client = KafkaClient(config.KAFKA_URL)
        kafka_client.consume(
            topic=config.KAFKA_TOPIC,
            group_id=config.KAFKA_GROUP_ID,
            on_message=lambda msg: process_message(msg, rate_limiter)
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