import sys
from base64 import b64decode

import click
import boto3
from botocore.exceptions import ProfileNotFound
import pkcs11


@click.command()
@click.option("-i", "--instance_id", prompt=True, required=True)
@click.option("-p", "--profile", default="default", show_default=True)
@click.option("-t", "--token", "token_serial", default=None, help="Token serial number")
@click.option(
    "-l",
    "--lib",
    default="/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so",
    show_default=True,
)
def main(instance_id, profile, token_serial, lib):
    lib = pkcs11.lib(lib)

    if token_serial is not None:
        token_serial = token_serial.encode()

    try:
        token = lib.get_token(token_serial=token_serial)
    except pkcs11.MultipleTokensReturned:
        click.echo("Multiple tokens found. Please provide token serial number.")
        click.echo("Available token serial numbers:")
        for token in lib.get_tokens():
            click.echo(token.serial)
        sys.exit()

    click.echo(f"Using token serial number: {token.serial.decode()}")

    try:
        boto_session = boto3.Session(profile_name=profile)
    except ProfileNotFound:
        click.echo(f'Could not find profile "{profile}"')
        sys.exit()

    client = boto_session.client("ec2")

    response = client.get_password_data(InstanceId=instance_id, DryRun=False)

    password_data = response["PasswordData"]

    if not password_data:
        click.echo("Instance is not ready yet.")
        sys.exit()

    password_data_raw = b64decode(password_data)

    pin = click.prompt("PIN", hide_input=True)

    with token.open(user_pin=pin) as session:
        for obj in session.get_objects({pkcs11.Attribute.LABEL: "PIV AUTH key"}):
            click.echo(
                obj.decrypt(
                    password_data_raw, mechanism=pkcs11.Mechanism.RSA_PKCS
                ).decode()
            )


if __name__ == "__main__":
    main()  # pylint: disable=no-value-for-parameter
