## problem

worktree in docker has 'missing link' problem

worktree is created from git repo

worktree only is mounted into container image as volume

worktree .git points with absolute path to unmounted git root

## solution

use --relative-paths option and mount the repo-root's .git directory in a way that the workspace can reference

container run -d -t --name rel-dir -v /Users/ptone/dev/cli-projects/qa-scion/.scion/agents/rel-dir/home:/home/node -v /Users/ptone/dev/cli-projects/qa-scion/:/repo-root --workdir /repo-root/.scion/agents/rel-dir/workspace -e GEMINI_API_KEY=AIzaSyCMUZPTej3N-dTs-bgfhCrPdYUgRiFw_FM -e GEMINI_DEFAULT_AUTH_TYPE=gemini-api-key -e GEMINI_MODEL=flash -v /Users/ptone/.config/gcloud:/home/node/.config/gcloud:ro -e GEMINI_AGENT_NAME=rel-dir --label scion.agent=true --label scion.name=rel-dir --label scion.grove=qa-scion --label scion.grove_path=/Users/ptone/dev/cli-projects/qa-scion/.scion --label scion.template=default --label scion.tmux=true gemini-cli-sandbox:tmux tmux new-session -s scion gemini --yolo --prompt-interactive "hello"

So the functional bit of git that creates the worktree should be something like:

    git worktree add --relative-paths "./workspace" "rel-dir"

And then the new container run command would have an extra mount compared to the one above it (part of the runtime pkg)

container run -d -t --name rel-dir -v /Users/ptone/dev/cli-projects/qa-scion/.scion/agents/rel-dir/home:/home/node -v /Users/ptone/dev/cli-projects/qa-scion/.git:/repo-root/.git -v /Users/ptone/dev/cli-projects/qa-scion/.scion/agents/rel-dir/workspace:/repo-root/.scion/agents/rel-dir/workspace --workdir / -e GEMINI_API_KEY=AIzaSyCMUZPTej3N-dTs-bgfhCrPdYUgRiFw_FM -e GEMINI_DEFAULT_AUTH_TYPE=gemini-api-key -e GEMINI_MODEL=flash -v /Users/ptone/.config/gcloud:/home/node/.config/gcloud:ro -e GEMINI_AGENT_NAME=rel-dir --label scion.agent=true --label scion.name=rel-dir --label scion.grove=qa-scion --label scion.grove_path=/Users/ptone/dev/cli-projects/qa-scion/.scion --label scion.template=default --label scion.tmux=true gemini-cli-sandbox:tmux tmux new-session -s scion gemini --yolo --prompt-interactive "hello"




