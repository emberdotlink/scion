#!/bin/bash
set -e

# Ensure Hub is enabled for the grove
cd /Users/ptone/src/cli-projects/qa-scion

scion hub enable

echo "=== Phase 2 Template Upload Walkthrough ==="

# 1. Create a test template directory
echo -e "\n[1] Creating test template..."
mkdir -p /tmp/test-template/home/.claude
cat > /tmp/test-template/scion-agent.yaml << 'EOF'
harness: claude
image: scion-claude:latest
EOF

cat > /tmp/test-template/home/.bashrc << 'EOF'
# Custom bashrc for test template
export PS1="[\u@\h \W]\$ "
alias ll='ls -la'
EOF

cat > /tmp/test-template/home/.claude/CLAUDE.md << 'EOF'
# Test Template Instructions
This is a test template created for Phase 2 QA.
EOF

echo "Created template files in /tmp/test-template/"
ls -la /tmp/test-template/

# 2. Sync template to Hub (creates and uploads)
echo -e "\n[2] Syncing template to Hub..."
scion template sync test-phase2 \
  --from /tmp/test-template \
  --harness claude \
  --scope grove

# 3. List templates to verify creation
echo -e "\n[3] Listing templates..."
scion template list

# 4. Modify local template and push changes
echo -e "\n[4] Modifying and pushing template..."
echo "# Updated content" >> /tmp/test-template/home/.claude/CLAUDE.md
scion template push test-phase2 --from /tmp/test-template

# 5. Pull template to a new location
echo -e "\n[5] Pulling template..."
rm -rf /tmp/test-template-pulled
scion template pull test-phase2 --to /tmp/test-template-pulled

# 6. Verify pulled content
echo -e "\n[6] Verifying pulled content..."
ls -la /tmp/test-template-pulled/
cat /tmp/test-template-pulled/home/.claude/CLAUDE.md

# 7. Compare original and pulled
echo -e "\n[7] Comparing files..."
diff /tmp/test-template/scion-agent.yaml /tmp/test-template-pulled/scion-agent.yaml && \
  echo "scion-agent.yaml matches" || echo "MISMATCH!"
diff /tmp/test-template/home/.bashrc /tmp/test-template-pulled/home/.bashrc && \
  echo "home/.bashrc matches" || echo "MISMATCH!"

echo -e "\n=== Walkthrough Complete ==="