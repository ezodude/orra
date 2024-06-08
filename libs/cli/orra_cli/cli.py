import importlib
import signal
import sys
from logging import getLogger
from typing import Annotated, Union

import typer
from motleycache import enable_cache, disable_cache, set_update_cache_if_exists

from . import __version__
from .exceptions import OrraCliException
from .logging import setup_logging
from .printer import RichPrinter
from .resolve import get_import_string

app = typer.Typer(rich_markup_mode="rich")

setup_logging()
logger = getLogger(__name__)


def signal_handler(sig, frame):
    disable_cache()
    sys.exit(0)


signal.signal(signal.SIGINT, signal_handler)

try:
    import uvicorn
except ImportError:  # pragma: no cover
    uvicorn = None  # type: ignore[assignment]


def version_callback(value: bool):
    if value:
        print(f"CLI Version: {__version__}")
        raise typer.Exit()


@app.callback(no_args_is_help=True)
def callback(
        version: Annotated[Union[bool, None], typer.Option(
            default="--version",
            help="Show the version and exit.",
            callback=version_callback,
            is_eager=True
        ),
        ] = None, ) -> None:
    """
    🪡   Orra - make your AI agents work better together!
    """


@app.command()
def run(
        cache: bool = typer.Option(False, "--cache", help="Cache LLM / tool calls and all web requests."),
        debug: bool = typer.Option(False, "--debug", help="Activate debug mode.")
) -> None:
    host = "127.0.0.1"
    port = 1430
    printer = RichPrinter()
    configure_caching_if_needed(cache, printer)

    try:
        orra_import = get_import_string(path=None, app_name="app")
    except OrraCliException as e:
        logger.error(str(e))
        raise typer.Exit(code=1) from None

    server_factory = compile_and_prep_orra(orra_import, printer, debug, host, port)

    if not uvicorn:
        raise OrraCliException("Could not import Uvicorn") from None

    uvicorn.run(
        app=server_factory,
        host=host,
        port=port,
        log_level=("debug" if debug else "info")
    )


def configure_caching_if_needed(cache: bool = False, printer: RichPrinter = None):
    if not cache:
        return

    set_update_cache_if_exists(False)
    enable_cache()
    printer.print('Initialising \[cache] mode...Done!')


def compile_and_prep_orra(orra_import, printer, debug, host=None, port=None):
    parts = orra_import.split(":")
    module = importlib.import_module(parts[0])
    orra_app = getattr(module, parts[1])
    factory = orra_app.compile(printer=printer, debug=debug)

    printer.print("Starting Orra application... Done!")
    printer.print("")
    printer.print("Orra development server running!")
    printer.print(f"Your API is running at:     http://{host}:{port}")
    printer.print("")

    return factory


def main():
    app()
