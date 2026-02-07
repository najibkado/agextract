import json
import re
from datetime import datetime
from .models import Session, Step

class TranscriptParser:
    def __init__(self, file_content):
        self.content = file_content.decode('utf-8') if isinstance(file_content, bytes) else file_content

    def parse(self, title="Uploaded Session"):
        # Auto-detect JSONL format (Claude Code)
        first_line = self.content.strip().split('\n')[0] if self.content.strip() else ''
        if first_line.startswith('{'):
            try:
                json.loads(first_line)
                return self._parse_jsonl(title)
            except (json.JSONDecodeError, ValueError):
                pass
        return self._parse_markdown(title)

    def _parse_jsonl(self, title):
        """Parse Claude Code JSONL format."""
        session = Session.objects.create(title=title)
        step_counter = 1

        for line in self.content.strip().split('\n'):
            line = line.strip()
            if not line:
                continue
            try:
                entry = json.loads(line)
            except (json.JSONDecodeError, ValueError):
                continue

            msg_type = entry.get('type', '')
            role = None
            step_type = 'text'
            content = ''

            if msg_type in ('human', 'user'):
                role = 'user'
                step_type = 'prompt'
                content = self._extract_jsonl_content(entry)
            elif msg_type in ('assistant', 'agent'):
                role = 'agent'
                step_type = 'text'
                content = self._extract_jsonl_content(entry)
            elif msg_type in ('tool_use', 'tool_call'):
                role = 'agent'
                step_type = 'tool_call'
                content = self._extract_jsonl_content(entry)
            elif msg_type == 'tool_result':
                role = 'system'
                step_type = 'text'
                content = self._extract_jsonl_content(entry)
            else:
                continue

            if content and role:
                Step.objects.create(
                    session=session, role=role, step_type=step_type,
                    content=content.strip(), order=step_counter,
                )
                step_counter += 1

        session.file_count = 1
        session.save()
        return session

    def _extract_jsonl_content(self, entry):
        """Extract text content from a JSONL entry."""
        # Try 'message' field first (Claude Code format)
        msg = entry.get('message', {})
        if isinstance(msg, dict):
            content = msg.get('content', '')
            if isinstance(content, list):
                # Content blocks: [{"type": "text", "text": "..."}, ...]
                parts = []
                for block in content:
                    if isinstance(block, dict):
                        if block.get('type') == 'text':
                            parts.append(block.get('text', ''))
                        elif block.get('type') == 'tool_use':
                            parts.append(f"Tool: {block.get('name', '')} — {json.dumps(block.get('input', {}))[:500]}")
                        elif block.get('type') == 'tool_result':
                            parts.append(str(block.get('content', ''))[:1000])
                return '\n'.join(parts)
            if isinstance(content, str):
                return content
        if isinstance(msg, str):
            return msg

        # Fallback to top-level 'content'
        content = entry.get('content', '')
        if isinstance(content, str):
            return content
        if isinstance(content, (list, dict)):
            return json.dumps(content)[:2000]
        return str(content) if content else ''

    def _parse_markdown(self, title):
        """
        Parses the content and saves it to the database.
        Returns the created Session object.
        """
        # Create the session
        session = Session.objects.create(title=title)
        
        # Split content into chunks based on headers
        # This is a naive implementation assuming "## Step", "## User", or similar structure
        # We'll refine this based on actual transcript formats.
        
        # Strategy: Iterate through lines, state machine approach
        lines = self.content.split('\n')
        current_role = None
        current_buffer = []
        step_counter = 1
        
        for line in lines:
            line_stripped = line.strip()
            # Check for role indicators
            user_match = re.match(r'^\s*(#+\s*)?(User|Human)(\s*Input)?:?', line, re.IGNORECASE)
            agent_match = re.match(r'^\s*(#+\s*)?(Agent|Assistant|AI):?', line, re.IGNORECASE)
            
            # Check for "Tool/System" lines (e.g. *Edited relevant file*)
            # We treat these as discrete single-line steps if they are standalone
            is_tool_line = line_stripped.startswith('*') and line_stripped.endswith('*') and len(line_stripped) > 2
            
            if user_match or agent_match:
                # Save previous step
                if current_role and current_buffer:
                    content = '\n'.join(current_buffer).strip()
                    if content:
                        self._create_step(session, current_role, content, step_counter)
                        step_counter += 1
                    current_buffer = []

                # Set new role
                current_role = 'user' if user_match else 'agent'
                
            elif is_tool_line:
                # This is a tool event. It implicitly breaks the flow.
                # If we were accumulating a previous step (User or Agent text), save it.
                if current_role and current_buffer:
                    content = '\n'.join(current_buffer).strip()
                    if content:
                        self._create_step(session, current_role, content, step_counter)
                        step_counter += 1
                    current_buffer = []
                
                # Create tool step
                # Explicitly pass step_type='tool_call' to helper if we could, 
                # but helper signature needs update or we rely on content check.
                # Let's rely on updated content check in helper.
                self._create_step(session, 'agent', line_stripped, step_counter)
                step_counter += 1
                
                current_role = 'agent' 
            else:
                # Accumulate content
                current_buffer.append(line)
        
        # Save the last step
        if current_role and current_buffer:
            content = '\n'.join(current_buffer).strip()
            if content:
                self._create_step(session, current_role, content, step_counter)

        # Update session metadata
        session.file_count = 1 # simplified
        session.save()
        
        return session

    def _create_step(self, session, role, content, order):
        """
        Helper to analyze content and create a Step object.
        """
        step_type = 'text' if role == 'agent' else 'prompt'
        
        # Detect step type based on content
        if '```diff' in content or '<<<<<<<' in content:
            step_type = 'diff'
        elif 'Tool Call' in content or '<function_calls>' in content:
            step_type = 'tool_call'
        elif content.startswith('*') and content.endswith('*'):
             # Handle s.md style tool calls
            step_type = 'tool_call'
        elif role == 'user':
            step_type = 'prompt'
            
        Step.objects.create(
            session=session,
            role=role,
            step_type=step_type,
            content=content.strip(),
            order=order
        )
