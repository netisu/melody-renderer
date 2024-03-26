## Aeo's Setup Guide: Aeno (renderer)

### Prerequisites:

Before proceeding with the installation, make sure you meet the following prerequisites:

1. **Operating System:** You can use either Ubuntu/Linux Server (version 20.04+) or a Windows machine with Laragon installed.
2. **Laragon:** If you're on a Windows machine, download Laragon from [Laragon.org](https://laragon.org).

### Installation Commands:

Execute the following commands to install the required packages:

```bash
sudo apt install php8.1-{memcache,fpm,cgi,http,raphf,memcached,common,redis,mysql,mysqli,sodium} zip unzip unrar nginx memcache curl && sudo apt remove apache*
```

## Nginx Setup

Configure Nginx by editing the default configuration file:

```bash
nano /etc/nginx/sites-available/default
```

Make the following modifications in the configuration:

```nginx
server {
    listen 80 default_server;
    listen [::]:80 default_server;
    server_name yourdomain.com;
    root /var/www/html/public;

    # Add security headers
    add_header X-Frame-Options "SAMEORIGIN";
    add_header X-Content-Type-Options "nosniff";

    index index.php;

    charset utf-8;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    # Other location blocks for specific routes

    location ~ \.php$ {
        fastcgi_pass unix:/var/run/php/php8.1-fpm.sock;
        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;
        include fastcgi_params;
    }

    # Additional security rules
}
```

Then restart the webserver.

```bash
sudo service nginx restart
```
Make sure you set the correct permissions!
```bash
chmod -R 777 /var/www/* && chmod -R 777 /var/www/html/* 
```
## Node.js Setup

Install the Node.js Version Manager (NVM) by following these steps:

1. Install nvm (Node.js Version manager):

```bash
curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.1/install.sh | bash
```

2. Apply fnm configuration to your shell:

### Run this in your shell:
```bash
export NVM_DIR="$([ -z "${XDG_CONFIG_HOME-}" ] && printf %s "${HOME}/.nvm" || printf %s "${XDG_CONFIG_HOME}/nvm")"
[ -s "$NVM_DIR/nvm.sh" ] && \. "$NVM_DIR/nvm.sh"
```
### Then:

```bash
source ~/.bashrc
```

3. Install the latest LTS version of Node.js:

```bash
nvm install node
```

## Composer and Dependencies

Install Composer and project dependencies:

1. Install Composer:

```bash
php -r "copy('https://getcomposer.org/installer', 'composer-setup.php');"
php -r "if (hash_file('sha384', 'composer-setup.php') === 'e21205b207c3ff031906575712edab6f13eb0b361f2085f1f1237b7126d785e826a450292b6cfd1d64d92e6563bbde02') { echo 'Installer verified'; } else { echo 'Installer corrupt'; unlink('composer-setup.php'); } echo PHP_EOL;"
php composer-setup.php
php -r "unlink('composer-setup.php');" && sudo mv composer.phar /usr/local/bin/composer
```

2. Install dependencies:

```bash
composer upgrade && npm upgrade
```

## Finalization

Finish setting up your environment:

1. Copy the `.env.example` file:

```bash
cd /var/www/html && cp .env.example .env
```

2. Edit the `.env` configuration:

```dotenv
# Configure your application settings here
```

3. Generate your application key:

```bash
php artisan key:generate
```

4. Configure Eclipse settings:

Edit the `Values.php` configuration file:

```bash
nano /var/www/html/config/Values.php
```

Modify the settings in this file as needed.

Remember, for other components like the database, renderer, games, security patches, and Git, please refer to the respective files in the /documentation/ folder.

With these steps completed, your setup is nearly finished, and you're ready to host your code.
