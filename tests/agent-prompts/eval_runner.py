#!/usr/bin/env python3
"""
USPTO CLI Skill Eval Runner

Runs each agent stress-test prompt through frix_headless.py and captures
structured eval results to JSON files.

Usage:
    python tests/agent-prompts/eval_runner.py                    # Run all 10 prompts
    python tests/agent-prompts/eval_runner.py --prompts 1,3,5    # Run specific prompts
    python tests/agent-prompts/eval_runner.py --timeout 600      # Custom timeout (default: 300s)
    python tests/agent-prompts/eval_runner.py --dry-run           # Print prompts without running

Output:
    tests/agent-prompts/results/prompt01.json ... prompt10.json
    tests/agent-prompts/results/eval_summary.json
"""

import subprocess
import sys
import os
import re
import json
import time
import argparse
from pathlib import Path
from datetime import datetime
from typing import Optional

# Paths
SCRIPT_DIR = Path(__file__).parent.resolve()
FRIX_ROOT = Path(r"C:\Users\sethc\dev\frix-agent")
FRIX_HEADLESS = FRIX_ROOT / "frix_headless.py"
RESULTS_DIR = SCRIPT_DIR / "results"
WORKSPACE_BASE = SCRIPT_DIR / "workspaces"

# Template for the structured output instruction (eval_file placeholder filled per-prompt)
OUTPUT_INSTRUCTION_TEMPLATE = """\

FINAL STEP — REQUIRED: When you have completed all tasks above, write your evaluation
results to the file below as valid JSON. Use the Write tool or bash to create this file.
This is mandatory — the eval runner reads this file to score your run.

File path: {eval_file}

Write this exact JSON schema (fill in your actual values):
{{
  "success": true or false,
  "turns": <number of CLI commands you executed>,
  "commands": ["uspto search ...", "uspto app claims ...", ...],
  "ax": "your experience using the CLI: what worked well, what was confusing, any friction",
  "suggestions": "suggestions to improve the CLI tool or the /uspto skill based on this task"
}}"""

# Constraint prepended to every prompt
CONSTRAINT = (
    "IMPORTANT: You may NOT use any Minesoft tools, patent-search tools, "
    "or any other patent data source. This is an evaluation of the "
    "uspto tool ONLY. All patent data must be retrieved exclusively "
    "via the `uspto` command-line tool."
)


def extract_prompt_text(md_path: Path) -> str:
    """Extract the raw prompt from a prompt markdown file.

    Looks for the ## The Prompt section and extracts the blockquote content.
    """
    text = md_path.read_text(encoding="utf-8")
    # Find the blockquote section under "## The Prompt"
    in_prompt = False
    lines = []
    for line in text.splitlines():
        if line.strip().startswith("## The Prompt"):
            in_prompt = True
            continue
        if in_prompt:
            if line.strip().startswith("## "):
                break  # next section
            # Strip blockquote prefix
            stripped = line.strip()
            if stripped.startswith("> "):
                lines.append(stripped[2:])
            elif stripped == ">":
                lines.append("")
            elif stripped == "" and lines:
                # Blank line after blockquote — probably end of quote
                # but only if next line isn't a blockquote
                lines.append("")
            elif lines and not stripped.startswith(">"):
                # Non-blockquote line after blockquote content = end
                if stripped:
                    break

    return "\n".join(lines).strip()


def build_full_prompt(prompt_text: str, prompt_num: int, eval_file: Path) -> str:
    """Build the full prompt with skill invocation, constraints, and file-based output."""
    output_instruction = OUTPUT_INSTRUCTION_TEMPLATE.format(eval_file=eval_file)
    return (
        f"/uspto\n\n"
        f"{CONSTRAINT}\n\n"
        f"--- BEGIN TASK (Prompt {prompt_num:02d}) ---\n\n"
        f"{prompt_text}\n\n"
        f"--- END TASK ---\n\n"
        f"{output_instruction}"
    )


def read_eval_file(eval_file: Path) -> Optional[dict]:
    """Read the structured eval result JSON that the agent wrote to disk."""
    if not eval_file.exists():
        return None
    try:
        text = eval_file.read_text(encoding="utf-8").strip()
        # Handle markdown code fences in case agent wrapped it
        if text.startswith("```"):
            lines = text.splitlines()
            # Strip first and last lines if they're fences
            if lines[0].startswith("```"):
                lines = lines[1:]
            if lines and lines[-1].strip().startswith("```"):
                lines = lines[:-1]
            text = "\n".join(lines).strip()
        return json.loads(text)
    except (json.JSONDecodeError, Exception):
        return None


def parse_eval_from_stdout(stdout: str) -> Optional[dict]:
    """Fallback: try to parse eval result from stdout if file wasn't written."""
    # Search for the marker line
    for line in stdout.splitlines():
        line = line.strip()
        if line.startswith("EVAL_RESULT_JSON:"):
            json_str = line[len("EVAL_RESULT_JSON:"):].strip()
            try:
                return json.loads(json_str)
            except json.JSONDecodeError:
                pass

    # Fallback: try to find any JSON block that matches our schema
    json_pattern = re.compile(
        r'\{[^{}]*"success"\s*:\s*(true|false)[^{}]*"commands"\s*:\s*\[.*?\][^{}]*\}',
        re.DOTALL
    )
    match = json_pattern.search(stdout)
    if match:
        try:
            return json.loads(match.group())
        except json.JSONDecodeError:
            pass

    return None


def run_prompt(
    prompt_num: int,
    prompt_text: str,
    full_prompt: str,
    eval_file: Path,
    timeout: int,
    verbose: bool
) -> dict:
    """Run a single prompt through frix_headless.py and return the result."""
    workspace = WORKSPACE_BASE / f"prompt{prompt_num:02d}"
    workspace.mkdir(parents=True, exist_ok=True)

    # Clean up any previous eval file
    if eval_file.exists():
        eval_file.unlink()

    result_record = {
        "prompt_number": prompt_num,
        "prompt_text": prompt_text,
        "timestamp": datetime.now().isoformat(),
        "duration_seconds": 0,
        "exit_code": None,
        "eval_result": None,
        "raw_stdout_tail": "",
        "raw_stderr_tail": "",
        "parse_error": None,
    }

    print(f"  [{prompt_num:02d}/10] Running...", end=" ", flush=True)
    start = time.time()

    try:
        env = os.environ.copy()
        env["PYTHONIOENCODING"] = "utf-8"

        proc = subprocess.run(
            [sys.executable, str(FRIX_HEADLESS), full_prompt, "--workspace", str(workspace), "--verbose"],
            capture_output=True,
            text=True,
            timeout=timeout,
            cwd=str(FRIX_ROOT),
            env=env,
            encoding="utf-8",
            errors="replace",
        )

        duration = time.time() - start
        result_record["duration_seconds"] = round(duration, 2)
        result_record["exit_code"] = proc.returncode
        result_record["raw_stdout_tail"] = proc.stdout[-3000:] if proc.stdout else ""
        result_record["raw_stderr_tail"] = proc.stderr[-1000:] if proc.stderr else ""

        # Try reading eval result from file first (preferred), then fall back to stdout
        eval_result = read_eval_file(eval_file)
        if not eval_result:
            eval_result = parse_eval_from_stdout(proc.stdout)

        if eval_result:
            result_record["eval_result"] = eval_result
            status = "PASS" if eval_result.get("success") else "FAIL"
        else:
            result_record["parse_error"] = (
                f"Agent did not write eval file to {eval_file.name} "
                "and no structured result found in stdout"
            )
            status = "NO_RESULT"

        print(f"{status:10} ({duration:.0f}s)")

        if verbose and eval_result:
            print(f"           turns={eval_result.get('turns', '?')}, "
                  f"commands={len(eval_result.get('commands', []))}")
            if eval_result.get("ax"):
                ax_short = eval_result["ax"][:120]
                print(f"           ax: {ax_short}")

    except subprocess.TimeoutExpired:
        duration = time.time() - start
        result_record["duration_seconds"] = round(duration, 2)
        result_record["exit_code"] = -1
        result_record["parse_error"] = f"Timed out after {timeout}s"
        # Still check if file was written before timeout
        eval_result = read_eval_file(eval_file)
        if eval_result:
            result_record["eval_result"] = eval_result
        print(f"TIMEOUT    ({timeout}s)")

    except Exception as e:
        duration = time.time() - start
        result_record["duration_seconds"] = round(duration, 2)
        result_record["exit_code"] = -2
        result_record["parse_error"] = str(e)
        print(f"ERROR      ({duration:.0f}s) {e}")

    return result_record


def save_log(prompt_num: int, full_prompt: str, result: dict):
    """Save the full log for a prompt run."""
    log_dir = RESULTS_DIR / "logs"
    log_dir.mkdir(parents=True, exist_ok=True)
    log_file = log_dir / f"prompt{prompt_num:02d}.log"

    with open(log_file, "w", encoding="utf-8") as f:
        f.write(f"Prompt {prompt_num:02d}\n")
        f.write(f"Timestamp: {result['timestamp']}\n")
        f.write(f"Duration: {result['duration_seconds']}s\n")
        f.write(f"Exit Code: {result['exit_code']}\n")
        f.write(f"\n{'='*70}\nFULL PROMPT:\n{'='*70}\n")
        f.write(full_prompt)
        f.write(f"\n\n{'='*70}\nSTDOUT (tail):\n{'='*70}\n")
        f.write(result.get("raw_stdout_tail", "(empty)"))
        f.write(f"\n\n{'='*70}\nSTDERR (tail):\n{'='*70}\n")
        f.write(result.get("raw_stderr_tail", "(empty)"))


def main():
    parser = argparse.ArgumentParser(description="USPTO CLI Skill Eval Runner")
    parser.add_argument(
        "--prompts", type=str, default=None,
        help="Comma-separated prompt numbers to run (e.g., 1,3,5). Default: all."
    )
    parser.add_argument(
        "--timeout", type=int, default=300,
        help="Timeout per prompt in seconds (default: 300)"
    )
    parser.add_argument(
        "--verbose", "-v", action="store_true",
        help="Show detailed output per prompt"
    )
    parser.add_argument(
        "--dry-run", action="store_true",
        help="Print the full prompts without running them"
    )
    args = parser.parse_args()

    # Discover prompt files
    prompt_files = sorted(SCRIPT_DIR.glob("[0-9][0-9]-*.md"))
    if not prompt_files:
        print(f"Error: No prompt files found in {SCRIPT_DIR}", file=sys.stderr)
        sys.exit(1)

    # Filter to requested prompts
    if args.prompts:
        requested = {int(n.strip()) for n in args.prompts.split(",")}
        prompt_files = [f for f in prompt_files if int(f.name[:2]) in requested]

    # Validate frix_headless.py exists
    if not FRIX_HEADLESS.exists():
        print(f"Error: frix_headless.py not found at {FRIX_HEADLESS}", file=sys.stderr)
        sys.exit(1)

    # Extract prompts and prepare eval file paths
    RESULTS_DIR.mkdir(parents=True, exist_ok=True)
    prompts = []
    for pf in prompt_files:
        num = int(pf.name[:2])
        text = extract_prompt_text(pf)
        eval_file = RESULTS_DIR / f"prompt{num:02d}_eval.json"
        full = build_full_prompt(text, num, eval_file)
        prompts.append((num, text, full, pf.name, eval_file))

    if args.dry_run:
        for num, text, full, fname, eval_file in prompts:
            print(f"\n{'='*70}")
            print(f"PROMPT {num:02d} ({fname})")
            print(f"{'='*70}\n")
            print(full)
        print(f"\n{len(prompts)} prompts prepared (dry-run, nothing executed)")
        return

    print(f"\n{'='*70}")
    print("USPTO CLI SKILL EVALUATION")
    print(f"{'='*70}")
    print(f"Prompts:   {len(prompts)}")
    print(f"Timeout:   {args.timeout}s per prompt")
    print(f"Results:   {RESULTS_DIR}")
    print(f"Workspace: {WORKSPACE_BASE}")
    print(f"{'='*70}\n")

    all_results = []
    total_start = time.time()

    for num, text, full, fname, eval_file in prompts:
        result = run_prompt(num, text, full, eval_file, args.timeout, args.verbose)
        all_results.append(result)

        # Save individual result JSON (without raw stdout/stderr for cleanliness)
        clean_result = {k: v for k, v in result.items() if not k.startswith("raw_")}
        result_file = RESULTS_DIR / f"prompt{num:02d}.json"
        with open(result_file, "w", encoding="utf-8") as f:
            json.dump(clean_result, f, indent=2)

        # Save full log
        save_log(num, full, result)

    total_duration = time.time() - total_start

    # Build summary
    passed = [r for r in all_results if (r.get("eval_result") or {}).get("success")]
    failed = [r for r in all_results if r.get("eval_result") and not r["eval_result"].get("success")]
    no_result = [r for r in all_results if not r.get("eval_result")]

    all_commands = []
    all_ax = []
    all_suggestions = []
    for r in all_results:
        er = r.get("eval_result") or {}
        all_commands.extend(er.get("commands", []))
        if er.get("ax"):
            all_ax.append(f"P{r['prompt_number']:02d}: {er['ax']}")
        if er.get("suggestions"):
            all_suggestions.append(f"P{r['prompt_number']:02d}: {er['suggestions']}")

    summary = {
        "evaluation": "uspto-skill",
        "timestamp": datetime.now().isoformat(),
        "total_prompts": len(all_results),
        "passed": len(passed),
        "failed": len(failed),
        "no_result": len(no_result),
        "pass_rate": f"{len(passed)/len(all_results)*100:.0f}%" if all_results else "0%",
        "total_duration_seconds": round(total_duration, 2),
        "unique_commands_used": sorted(set(all_commands)),
        "total_command_invocations": len(all_commands),
        "agent_experience": all_ax,
        "agent_suggestions": all_suggestions,
        "per_prompt": [
            {
                "prompt": r["prompt_number"],
                "success": (r.get("eval_result") or {}).get("success"),
                "turns": (r.get("eval_result") or {}).get("turns"),
                "commands_count": len((r.get("eval_result") or {}).get("commands", [])),
                "duration_seconds": r["duration_seconds"],
            }
            for r in all_results
        ],
    }

    summary_file = RESULTS_DIR / "eval_summary.json"
    with open(summary_file, "w", encoding="utf-8") as f:
        json.dump(summary, f, indent=2)

    # Print summary
    print(f"\n{'='*70}")
    print("EVALUATION SUMMARY")
    print(f"{'='*70}")
    print(f"Passed:     {len(passed)}/{len(all_results)}")
    print(f"Failed:     {len(failed)}/{len(all_results)}")
    print(f"No Result:  {len(no_result)}/{len(all_results)}")
    print(f"Duration:   {total_duration:.0f}s total")
    print(f"Commands:   {len(all_commands)} invocations, {len(set(all_commands))} unique")
    print(f"{'='*70}")
    print(f"\nResults:    {RESULTS_DIR}")
    print(f"Summary:    {summary_file}")

    if no_result:
        print(f"\nPrompts with no parseable result:")
        for r in no_result:
            print(f"  P{r['prompt_number']:02d}: {r.get('parse_error', 'unknown')}")

    if all_suggestions:
        print(f"\nAgent Suggestions:")
        for s in all_suggestions[:5]:
            print(f"  {s[:150]}")

    print()


if __name__ == "__main__":
    main()

