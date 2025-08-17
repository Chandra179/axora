"""
Entrypoint: load config, init logging/metrics/tracing, 
create Kafka producer/consumer clients, start the worker loop, 
handle graceful shutdown. Exposes start_app() and stop_app().


"""