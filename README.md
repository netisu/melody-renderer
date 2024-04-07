## Aeo's Setup Guide: Aeno (renderer)

### Prerequisites:

Before proceeding with the installation, make sure you meet the following prerequisites:

1. **Operating System:** You can use either Ubuntu/Linux Server (version 20.04+) or a Windows machine with Laragon installed.
2. **Laragon:** If you're on a Windows machine, download Laragon from [Laragon.org](https://laragon.org).

You will also need to have golang installed.


### Installation Commands:

Execute the following commands to install the required packages:

```bash
sudo apt install php8.1-{memcache,fpm,cgi,http,raphf,memcached,common,redis,mysql,mysqli,sodium} zip unzip unrar nginx memcache curl && sudo apt remove apache*
```

Modify the settings in this file as needed.

Remember, for other components like the database, renderer, games, security patches, and Git, please refer to the respective files in the /documentation/ folder.

With these steps completed, your setup is nearly finished, and you're ready to host your code.
