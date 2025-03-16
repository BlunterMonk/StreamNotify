### Stream Notifier ###

This app will simply send a Windows 10 toast notification whenever a registered stream goes live.

### Install ###

Create this directory in your roaming folder
```
%APPDATA%/Roaming/StreamNotify/
```

Then move the 'config.json' into this folder
Currently there is no installer, so simply run the application on its own.
If you want to run the app as a windows service 
(this will allow you to run it automatically at startup and keep better track of the lifecycle)
you can use NSSM: instructions can be found here
https://nssm.cc/


### Features ###

- detect when livestreams go live and play them in browser

### config ###

The app relies on a config.json which must be present in the following directory
```
%APPDATA%/Roaming/StreamNotify/config.json
```

config values:
```
{
    "liveTimer": <time in minutes when live streams should be checked>
    "channels": {
        "<name>": "<channel_id>"
        ...
    }
}
```