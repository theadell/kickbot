# Slackbot for Football Table Games
A simple slackbot for managing games at workspace. 

```
[Idle]
  │
  │ User sends '/kicker' or '/kicker1v1'
  │ Bot sends game initiation message
  └───► [GameFormation]
        │
        ├─► Player joins : 'GAME_JOIN' interaction
        │   Bot updates game message with new player
        │
        ├─► Player leaves : 'GAME_LEAVE' interaction
        │   Bot updates game message, removes player
        │
        ├─► Quorum is reached
        │   Bot sends game created & ready message
        │   └───► [Idle]
        │         Ready for new game
        │
        └─► If all players leave
              Bot deletes game message
              └───► [Idle]
                    Ready for new game
```