from rich import print as rprint


class RichPrinter:
    @staticmethod
    def print(message: str) -> None:
        if message.lower().endswith("done!"):
            rprint(f"  [green]✔ {message}[/green]")
        else:
            rprint(f"  {message}")
