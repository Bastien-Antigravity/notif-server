#// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\
#       // ! \\         No underscore in label field name !!!                        // ! \\
#       // ! \\         field name lowercase                                         // ! \\
#       // ! \\         field number start = 0                                       // ! \\
#       // ! \\         enum number start = 0                                        // ! \\
#// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\// ! \\


# generate notifieMsg ncap proto
# ncap install is required :
# go get -u -t zombiezen.com/go/capnproto2
# GO111MODULE=off go get -u capnproto.org/go/capnp/v3/

# cd "/users/IMac/Desktop/govenv/api/capnp/notifieMsg"
# capnp compile -I "/users/IMac/Desktop/govenv/api/capnp/notifieMsg/go-capnp/std" -ogo notifie.capnp


using Go = import "/go.capnp";
@0xcd0e7dad96752db7;
$Go.package("Notifie");
$Go.import("govenv/pkg/common/Notifie");

struct NotifieMsg {
  message @0 :Text;
  attachment @1 :Text = "";
  tags @2 :List(Text) = ["telegram"];
}

