"""
Entrypoint: load config, init logging/metrics/tracing, 
create Kafka producer/consumer clients, start the worker loop, 
handle graceful shutdown. Exposes start_app() and stop_app().
"""

import logging
import signal
import sys
import time
from typing import Optional

from config import config


class CrawlerApp:
    """Main crawler application that coordinates all components."""
    
    def __init__(self):
        self.running = False
        self.kafka_consumer = None
        self.kafka_producer = None
        self.worker = None
        self.logger = None
        
    def _setup_logging(self):
        """Initialize logging configuration."""
        log_config = config.logging
        
        logging.basicConfig(
            level=getattr(logging, log_config.get('level', 'INFO')),
            format=log_config.get('format', '%(asctime)s - %(name)s - %(levelname)s - %(message)s'),
            handlers=[logging.StreamHandler(sys.stdout)]
        )
        
        self.logger = logging.getLogger(__name__)
        self.logger.info("Logging initialized")
    
    def _setup_signal_handlers(self):
        """Setup graceful shutdown signal handlers."""
        def signal_handler(signum, frame):
            self.logger.info(f"Received signal {signum}, initiating graceful shutdown...")
            self.stop_app()
        
        signal.signal(signal.SIGINT, signal_handler)
        signal.signal(signal.SIGTERM, signal_handler)
    
    def _init_kafka_clients(self):
        """Initialize Kafka producer and consumer clients."""
        try:
            kafka_config = config.kafka
            self.logger.info(f"Initializing Kafka clients with bootstrap servers: {kafka_config.get('bootstrap_servers')}")
            
            # Import and initialize Kafka clients here
            # from consumer import Consumer
            # from producer import Producer
            # 
            # self.kafka_consumer = Consumer(kafka_config)
            # self.kafka_producer = Producer(kafka_config)
            
            self.logger.info("Kafka clients initialized successfully")
            
        except Exception as e:
            self.logger.error(f"Failed to initialize Kafka clients: {e}")
            raise
    
    def _init_worker(self):
        """Initialize the crawler worker."""
        try:
            # Import and initialize worker here
            # from worker import Worker
            # 
            # self.worker = Worker(
            #     consumer=self.kafka_consumer,
            #     producer=self.kafka_producer,
            #     config=config
            # )
            
            self.logger.info("Worker initialized successfully")
            
        except Exception as e:
            self.logger.error(f"Failed to initialize worker: {e}")
            raise
    
    def _init_metrics(self):
        """Initialize metrics collection if enabled."""
        metrics_config = config.metrics
        
        if not metrics_config.get('enabled', False):
            self.logger.info("Metrics collection disabled")
            return
        
        try:
            # Initialize metrics server here
            # from prometheus_client import start_http_server
            # 
            # port = metrics_config.get('port', 8080)
            # start_http_server(port)
            # self.logger.info(f"Metrics server started on port {port}")
            
            self.logger.info("Metrics collection would be initialized here")
            
        except Exception as e:
            self.logger.error(f"Failed to initialize metrics: {e}")
            raise
    
    def start_app(self):
        """Start the crawler application."""
        try:
            self.logger.info("Starting Axora Crawler...")
            
            # Log configuration summary
            self.logger.info(f"MongoDB URI: {config.mongodb.get('uri')}")
            self.logger.info(f"Kafka Bootstrap Servers: {config.kafka.get('bootstrap_servers')}")
            self.logger.info(f"User Agent: {config.fetcher.get('user_agent')}")
            self.logger.info(f"Politeness Delay: {config.politeness.get('default_delay')}s")
            
            # Initialize components
            self._init_kafka_clients()
            self._init_worker()
            self._init_metrics()
            
            # Start the main worker loop
            self.running = True
            self.logger.info("Crawler started successfully, entering worker loop...")
            
            self._worker_loop()
            
        except Exception as e:
            self.logger.error(f"Failed to start crawler: {e}")
            sys.exit(1)
    
    def _worker_loop(self):
        """Main worker loop that processes crawl tasks."""
        crawler_config = config.crawler
        loop_delay = crawler_config.get('worker_loop_delay', 0.1)
        
        while self.running:
            try:
                # Main worker processing would happen here
                # self.worker.process_batch()
                
                time.sleep(loop_delay)
                
            except KeyboardInterrupt:
                self.logger.info("Received keyboard interrupt, shutting down...")
                break
            except Exception as e:
                self.logger.error(f"Error in worker loop: {e}")
                time.sleep(1)  # Back off on error
    
    def stop_app(self):
        """Stop the crawler application gracefully."""
        if not self.running:
            return
            
        self.logger.info("Stopping crawler...")
        self.running = False
        
        # Cleanup resources
        if self.kafka_consumer:
            try:
                # self.kafka_consumer.close()
                self.logger.info("Kafka consumer closed")
            except Exception as e:
                self.logger.error(f"Error closing Kafka consumer: {e}")
        
        if self.kafka_producer:
            try:
                # self.kafka_producer.close()
                self.logger.info("Kafka producer closed")
            except Exception as e:
                self.logger.error(f"Error closing Kafka producer: {e}")
        
        self.logger.info("Crawler stopped successfully")


def main():
    """Main entry point for the crawler application."""
    app = CrawlerApp()
    
    try:
        # Setup logging first
        app._setup_logging()
        
        # Setup signal handlers for graceful shutdown
        app._setup_signal_handlers()
        
        # Start the application
        app.start_app()
        
    except Exception as e:
        if app.logger:
            app.logger.error(f"Fatal error: {e}")
        else:
            print(f"Fatal error during startup: {e}")
        sys.exit(1)


if __name__ == "__main__":
    main()