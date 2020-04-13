import sys
from base64 import b64decode

import click
import boto3
import pkcs11


@click.command()
@click.option("-i", "--instance_id", prompt=True, required=True)
@click.option("-p", "--profile", default="default", show_default=True)
@click.option(
    "-l",
    "--lib",
    default="/usr/lib/x86_64-linux-gnu/opensc-pkcs11.so",
    show_default=True,
)
def main(instance_id, profile, lib):
    lib = pkcs11.lib(lib)
    token = lib.get_token()
    boto_session = boto3.Session(profile_name=profile)
    client = boto_session.client("ec2")

    response = client.get_password_data(InstanceId=instance_id, DryRun=False,)

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
