1. Navigate to https://discord.com/developers/applications and create New Application (top right). Name it whatever you like.

2. Click "Bot" on the left panel, then click the button on the right to Add Bot. Scroll down to the section titled `Privileged Gateway Intents`, and toggle the option for `Server Members Intent` to ensure it is enabled, then Save Changes.

3. Scroll up to where the Bot Icon is displayed. Change its Username to whatever you like (Such as Among Us). Optionally, you can replace the icon with one provided in this repo under the [images folder](https://github.com/denverquane/amongusdiscord/tree/master/images). **But make sure to Copy the `Token` on the right, and paste it to a safe location.** We will need it later in the installation steps; this is the `DISCORD_BOT_TOKEN` in the `sample.env` file.

4. On the left panel, click "OAuth2", and then check the box marked `bot` under `Scopes`. Then scroll down to `Bot Permissions`, and check the boxes marked `View Channels`, `Send Messages`, and `Mute Members` (or just `Administrator`, but be very careful doing this in general...).

5. Scroll back up to `Scopes`, and copy the URL in the field that begins with `https://discord.com/api/oauth2/authorize?`. Paste this in a new browser tab, and grant the App access to whatever server you wish it to access. Close this tab when Finalized.

6. Last step, almost there! Now we need to get the `DISCORD_GUILD_ID` and the `DISCORD_CHANNEL_ID`. Go to https://discord.com/app, and navigate to the Server you want the Bot to communicate in (Servers are also known as Guilds). Navigate to the text channel you will want the Bot to send messages and receive commands in, and look at the URL in your browser. It will have the format `https://discord.com/channels/<DISCORD_GUILD_ID>/<DISCORD_CHANNEL_ID>`. Use these ID fields to populate the `sample.env` in the installation steps below (or paste the IDs somewhere else for now, making sure to label them appropriately). If this text channel is private or limited to certain roles, you will need to manually grant access to the Bot.
