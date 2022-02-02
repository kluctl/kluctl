#!/usr/bin/env python
import logging
import sys
from concurrent import futures

import click
import grpc

import jinja2_server_pb2_grpc
from jinja2_server import Jinja2Servicer

logger = logging.getLogger(__name__)


@click.group()
@click.option("--debug", is_flag=True)
def cli(debug):
    level = logging.INFO
    if debug:
        level = logging.DEBUG
    logging.basicConfig(level=level)
    pass

@cli.command()
def serve():
    executor = futures.ThreadPoolExecutor(max_workers=10)
    server = grpc.server(executor)
    jinja2_server_pb2_grpc.add_Jinja2ServerServicer_to_server(Jinja2Servicer(), server)

    port = server.add_insecure_port('[::]:0')
    try:
        server.start()
        click.echo("%d" % port)
        server.wait_for_termination()
    except Exception as e:
        logger.warning("Failed to start server: %s", e)
        sys.exit(1)

if __name__ == "__main__":
    cli()
