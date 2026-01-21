#!/usr/bin/env python3
import argparse
import json
import sys
import os
from datetime import datetime

def main():
    parser = argparse.ArgumentParser(description="Generate report from artifacts")
    parser.add_argument("--title", default="Security Scan Report")
    parser.add_argument("--output", default="report.md")
    parser.add_argument("files", nargs="*", help="JSON result files to include")
    args = parser.parse_args()

    report = f"# {args.title}\n\n"
    report += f"**Date:** {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n\n"

    if not args.files:
        report += "No input files provided. This is a placeholder report.\n"
    
    for fpath in args.files:
        try:
            with open(fpath, 'r') as f:
                data = json.load(f)
            report += f"## File: {os.path.basename(fpath)}\n\n"
            report += "```json\n"
            report += json.dumps(data, indent=2)
            report += "\n```\n\n"
        except Exception as e:
            report += f"## Error reading {fpath}\n\n{str(e)}\n\n"

    # In a real app we might want to write to a file, but for the CLI/TUI feedback loop
    # printing to stdout is often better unless explicitly told to write a file.
    # However, 'report' implies a persistent artifact.
    # Let's write to file AND print a summary.
    
    try:
        with open(args.output, "w") as f:
            f.write(report)
        print(f"Report generated: {args.output}")
        print("Summary:")
        print(f"  Title: {args.title}")
        print(f"  Files included: {len(args.files)}")
    except Exception as e:
        print(f"Failed to write report: {e}", file=sys.stderr)
        sys.exit(1)

if __name__ == "__main__":
    main()
