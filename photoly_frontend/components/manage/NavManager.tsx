import { VStack, Button } from "@chakra-ui/react";
import { AiOutlineHome, AiOutlineTag, AiOutlineUser } from "react-icons/ai";
import { RiFileSettingsLine, RiGalleryLine } from "react-icons/ri";
import React, { useEffect } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";

const NavManager = ({
  changeSelection,
}: {
  changeSelection: (num: number) => void;
}) => {
  const token = useToken();
  const { get } = useApi();
  const [admin, setAdmin] = React.useState<boolean>(false);

  const getAuth = async () => {
    const response = await get("/admin/isAdmin", {
      headers: { "HRD-token": token },
    });
    if (!!response && response.data.msgCode === 200) {
      setAdmin(response.data.t);
    }
  };
  useEffect(() => {
    if (!!token) {
      getAuth();
    }
  }, [token]);
  return (
    <>
      <VStack spacing={0} bg={"white"} h={"calc(100%-4rem)"} w={"15vw"}>
        <Button
          leftIcon={<AiOutlineHome />}
          colorScheme="teal"
          variant="ghost"
          padding={6}
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(0);
          }}
        >
          My Profile
        </Button>
        <Button
          leftIcon={<AiOutlineTag />}
          colorScheme="teal"
          variant="ghost"
          padding={6}
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(1);
          }}
        >
          Tag Settings
        </Button>
        <Button
          leftIcon={<RiGalleryLine />}
          colorScheme="teal"
          variant="ghost"
          padding={6}
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(2);
          }}
        >
          Gallery Settings
        </Button>
        {admin ? (
          <>
            <Button
              leftIcon={<RiFileSettingsLine />}
              colorScheme="teal"
              variant="ghost"
              padding={6}
              minW="100%"
              justifyContent="flex-start"
              onClick={() => {
                changeSelection(3);
              }}
            >
              Admin Settings
            </Button>
            <Button
              leftIcon={<AiOutlineUser />}
              colorScheme="teal"
              variant="ghost"
              padding={6}
              minW="100%"
              justifyContent="flex-start"
              onClick={() => {
                changeSelection(4);
              }}
            >
              Admin User Settings
            </Button>
          </>
        ) : (
          ""
        )}
      </VStack>
    </>
  );
};
export default NavManager;
