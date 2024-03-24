# ezBadminton Server

The backend for the ezBadminton tournament organizer app.

## PocketBase

The server uses [PocketBase](https://pocketbase.io/) as a framework. PocketBase brings an SQLite database, authentication service, a Record API, realtime updates and file storage.

The code in this repository extends PocketBase using its various hooks to modify the behaviour of existing API routes, aswell as to add new ones.

The ezBadminton clients subscribe to the realtime updates to keep in
sync with all changes.

## Contributing

Everyone is welcome to fork and/or make pull requests!

### How to get started

- Install [Go 1.21+](https://go.dev/doc/install) on your system

- Create a working directory like `ez_badminton_server`

- Fork and clone this repository
    ```console
    you@yourdevice:~/ez_badminton_server$ git clone [your-forked-repository]
    ```

- Download the dependencies
     ```console
    you@yourdevice:~/ez_badminton_server$ go mod init github.com/ezBadminton/ezBadmintonServer && go mod tidy
    ```

- Compile and run the server
     ```console
    you@yourdevice:~/ez_badminton_server$ go run . serve
    ```
    It will create the `pb_data` directory.

- Stop the service (`Ctrl+C`)

	> **_NOTE:_** From this point the instructions are the same as in the [ezBadminton admin app](https://github.com/ezBadminton/ezBadmintonAdmin) contribution guide.

- Set up your admin access
    ```console
	you@yourdevice:~/ez_badminton_server$ go run . admin create test@example.com your-password
    ```
- Start the service again
     ```console
    you@yourdevice:~/ez_badminton_server$ go run . serve
    ```
- Open the [pocketbase admin UI](http://127.0.0.1:8090/_/) in your browser and log in

- Open the [pocketbase settings](http://127.0.0.1:8090/_/#/settings/import-collections) and import the ezBadminton database schema from [pb_schema.json](https://gist.githubusercontent.com/Snonky/1a596069391fb06eb3d916934e8c140b/raw/pb_schema.json).
  - You should be able to see the collection tables on the [pocketbase home page](http://127.0.0.1:8090/_/) now

- Select the 'tournament_organizer' user-collection and create a test-user for yourself
    - Click 'New Record', fill out the form and click 'Create'

- Select the 'tournaments' collection and create a tournament
    - Click 'New Record', give it a title and click 'Create'

You are ready to connect to the server (e.g. using the [ezBadminton Admin App](https://github.com/ezBadminton/ezBadmintonAdmin)!

To compile the server into an executable use
```console
you@yourdevice:~/ez_badminton_server$ go build .
```