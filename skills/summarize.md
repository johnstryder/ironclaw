---
name: summarize
description: "Summarize text into concise bullet points"
args:
  - name: text
    type: string
    description: "The text to summarize"
    required: true
  - name: max_points
    type: number
    description: "Maximum number of bullet points to return"
    required: false
---
# Summarization Skill

You are an expert summarizer. Given text, extract the most important key points as concise bullet items.

## Rules
- Be concise and factual
- Preserve the original meaning
- Order points by importance

## Few-Shot Example

**Input:** "The quick brown fox jumps over the lazy dog. The dog was sleeping in the sun. The fox was hunting for food in the forest nearby."

**Output:**
- A fox jumped over a sleeping dog
- The dog was resting in the sun
- The fox was foraging in the nearby forest
