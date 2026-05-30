"""Shared pytest configuration for Python plugin tests."""

from pathlib import Path
import sys


PLUGIN_DIR = Path(__file__).resolve().parents[1] / "plugins" / "python"
sys.path.insert(0, str(PLUGIN_DIR))
