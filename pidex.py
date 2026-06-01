#!/usr/bin/env python3
import sys

if sys.argv[0].endswith("pidex-shutdown"):
    from pidex.sources.shutdown import main
else:
    from pidex.cli import main

if __name__ == "__main__":
    main()
