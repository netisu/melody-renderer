## Aeo's Setup Guide: Aeno (renderer)

### Prerequisites:

Before proceeding with the installation, make sure you meet the following prerequisites:

1. **Operating System:** You can use either Ubuntu/Linux Server (version 20.04+) or a Windows machine with Laragon installed.
2. **Laragon:** If you're on a Windows machine, download Laragon from [Laragon.org](https://laragon.org).
3. **Golang:** You need to have golang installed.
4. **Supervisor:** To manage and automate the render service we need supervisor

You will also need to have golang installed.


### Installation Commands:

# Supervisor Config:

```bash                            
[program:netisu-renderer]
process_name=%(program_name)s_%(process_num)02d
directory=/usr/local
command=/usr/local/bin/renderer
autostart=true
autorestart=true
user=root
numprocs=1
redirect_stderr=true
stdout_logfile=/var/www/renderer/worker.log

[inet_http_server]
port = 127.0.0.1:4315
```

Modify the settings in this file as needed.

Remember, for other components like the database, renderer, games, security patches, and Git, please refer to the respective files in the /documentation/ folder.

With these steps completed, your setup is nearly finished, and you're ready to host your code.
