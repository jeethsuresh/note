# NOTE 

A simple indexed note taking app with potential for cloud backups and team support.

Initially created during an internal SCWX hackathon (Sept. 2020)

## Usage 

1. Build the exec and add it into one of the directories in your $PATH 
2. `note init` creates the database 
3. `note edit <name>` creates/edits documents with a specific name
4. `note search <search terms>` searches documents for specific words 
5. `note ls` lists all notes


## TODO 

- `note sync` to back things up and fetch what everyone else has uploaded/written 
- `note config` to manage configuration details 
- `note help` for helpful tips 
- `note init --index` to reset and reindex all your existing notes. right now if you re-run init after the initial creation it just wipes the DB indiscriminately.
