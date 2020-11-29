# Privacy
AutoMuteUs takes your privacy seriously. We will *never* distribute your data to any other parties, and user data is explicitly
not for sale or redistribution.

Using AutoMuteUs in your Discord server (or as a general user) falls under "Legitimate Interest" in [Article 6(1)(f) of the
General Data Protection Regulation](https://eur-lex.europa.eu/legal-content/EN/TXT/?qid=1528874672298&uri=CELEX:02016R0679-20160504) (GDPR).

1. We use data collection to display and aggregate statistics about what games a Discord User has played in Among Us.
2. We only use the minimal amount of data/PII necessary to generate and process these statistics.
3. Users can opt-out of data collection at any time if they don't wish for AutoMuteUs to gather this data. (`.au priv optout`)

# What Data does AutoMuteUs collect?
AutoMuteUs collects a very small amount of user information for statistics. Your Discord UserID, and any in-game names you have used
are the only Personally-Identifiable Information (PII) that the bot requires to gather statistics. All other data collected by AutoMuteUs
is non-identifiable in-game data, such as player color, crewmate/imposter role, etc. An example of a game data record recorded
by AutoMuteUs is shown below:
```
{
    "color": 11,
    "name": "Soup",
    "isAlive": true
}
```

AutoMuteUs uses a mapping of Discord UserIDs to arbitrary numerical IDs, which are used for correlating game events. If you
choose to delete the data that AutoMuteUs stores about you (with `.au privacy optout`), the mapping to your User ID is removed,
and the full history of your past games is deleted. Because of this, re-opting into data collection with AutoMuteUs (`.au privacy optin`) means
your past games and game events **are not recoverable**. Please carefully consider this before opting out, if you plan to
view your game statistics at any point in the future!

Questions and concerns about your Data Collection and Privacy can be addressed to gdpr@automute.us