---
name: Provider plugin
about: Add support for a new LLM vendor (Gemini, Bedrock, Mistral, etc.)
title: "provider: "
labels: ["enhancement", "provider"]
assignees: []
---

## Vendor

<!-- e.g. Google Gemini, Amazon Bedrock, Mistral AI -->

## API reference

<!-- Link to the API docs for the streaming and non-streaming response formats -->

## Wire format

**Streaming:** <!-- SSE / chunked JSON / other -->
**Token fields:** <!-- Where input/output/cached tokens appear in the response -->
**Cost data:** <!-- Is pricing published? Link if so -->

## Tools that use this vendor

<!-- e.g. Gemini CLI, aider --model gemini/... -->

## Would you like to implement it?

- [ ] Yes — I'll open a PR following the plugin guide
- [ ] No — I'm filing this for someone else to pick up

## Draft detection logic

<!-- Optional: how should Detect(req) identify this vendor? Host? Path? Header? -->
