import importlib
from logging import getLogger
from typing import Annotated, Union

import typer
from rich import print as rprint

from . import __version__
from .logging import setup_logging
from .exceptions import OrraCliException
from .resolve import get_import_string

app = typer.Typer(rich_markup_mode="rich")

setup_logging()
logger = getLogger(__name__)


class RichPrinter:
    def print(self, message: str) -> None:
        if message.lower().endswith("done!"):
            rprint(f"  [green]✔ {message}[/green]")
        else:
            rprint(f"  {message}")


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
    Orra - orra is great!
    """


# ✔ Building Encore application graph... Done!
# ✔ Analyzing service topology... Done!
# ✔ Creating PostgreSQL database cluster... Done!
# ✔ Generating boilerplate code... Done!
# ✔ Compiling application source code... Done!
# ✔ Running database migrations... Done!
# ✔ Starting Encore application... Done!
#
# Encore development server running!
#
# Your API is running at:     http://127.0.0.1:4000
# Development Dashboard URL:  http://localhost:9400/w2ghg
#
# 3:57PM INF registered API endpoint endpoint=Shorten path=/url service=url
# 3:57PM INF registered API endpoint endpoint=Get path=/url/:id service=url
# 3:57PM INF listening for incoming HTTP requests


@app.command()
def run() -> None:
    host = "127.0.0.1"
    port = 1430

    try:
        orra_import = get_import_string(path=None, app_name="app")
    except OrraCliException as e:
        logger.error(str(e))
        raise typer.Exit(code=1) from None

    printer = RichPrinter()
    parts = orra_import.split(":")

    module = importlib.import_module(parts[0])
    orra_app = getattr(module, parts[1])
    server_factory = orra_app.run(printer=printer)

    printer.print("Starting Orra application... Done!")

    printer.print("")
    printer.print("Orra development server running!")
    printer.print(f"Your API is running at:     http://{host}:{port}")
    printer.print("")

    if not uvicorn:
        raise OrraCliException(
            "Could not import Uvicorn"
        ) from None

    uvicorn.run(
        app=server_factory,
        host=host,
        port=port,
        log_level="info"
    )


def main():
    app()
