from kafka import KafkaConsumer
from typing import Callable
from kafka.errors import KafkaError
import json
import structlog

logger = structlog.get_logger(__name__)


class KafkaClient:
    def __init__(self, bootstrap_servers: str):
        self.bootstrap_servers = bootstrap_servers
        self.consumer = None
    
    def new_client(self, topic: str, group_id: str) -> KafkaConsumer:
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
            logger.info("kafka_client_connected", 
                       bootstrap_servers=self.bootstrap_servers,
                       topic=topic,
                       group_id=group_id)
            return self.consumer
        except Exception as e:
            logger.error("kafka_client_creation_failed", 
                        bootstrap_servers=self.bootstrap_servers,
                        error=str(e),
                        exc_info=True)
            raise
    
    
    def consume(self, topic: str, group_id: str, on_message: Callable):
        consumer = self.new_client(topic, group_id)
        
        try:
            while True:
                msg_pack = consumer.poll(timeout_ms=1000)
                
                for _, messages in msg_pack.items():
                    for message in messages:
                        try:
                            on_message(message.value)
                            
                        except Exception as e:
                            logger.error("message_processing_error", 
                                       topic=message.topic,
                                       partition=message.partition,
                                       offset=message.offset,
                                       error=str(e),
                                       exc_info=True)
        
        except KeyboardInterrupt:
            logger.info("consumer_shutdown_requested")
        
        except KafkaError as e:
            logger.error("kafka_fetch_error", 
                        error=str(e),
                        exc_info=True)
        
        finally:
            consumer.close()
            logger.info("consumer_closed")