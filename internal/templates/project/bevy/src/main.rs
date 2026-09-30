use bevy::prelude::*;
use mugon_sdk_bevy::prelude::*;

fn main() {
    App::new()
        // `MugonPlugin` reads the context that `Mugon.start` published, so the app has to be
        // started from inside its callback (see src/index.js).
        .add_plugins((DefaultPlugins, MugonPlugin))
        .add_systems(Startup, setup)
        .run();
}

fn setup(mut commands: Commands, bridge: NonSend<Bridge>) {
    commands.spawn(Camera2d);
    info!(
        "Running as {:?} with id {}",
        bridge.network_mode(),
        bridge.own_id()
    );

    // The platform shows a loading overlay until the game says it is playable. A game with
    // assets adds `MugonLoadingPlugin` and tracks them, and the overlay closes by itself once
    // they are loaded:
    //   loading.track(asset_server.load::<Image>("sprite.png"));   // ResMut<MugonLoading>
    bridge.loaded();
}
