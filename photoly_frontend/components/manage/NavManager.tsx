import { VStack, Button ***REMOVED*** from "@chakra-ui/react";
import { AiOutlineHome, AiOutlineTag, AiOutlineUser ***REMOVED*** from "react-icons/ai";
import { RiFileSettingsLine, RiGalleryLine ***REMOVED*** from "react-icons/ri";
import React, { useEffect ***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";

const NavManager = ({
  changeSelection,
***REMOVED***: {
  changeSelection: (num: number) => void;
***REMOVED***) => {
  const token = useToken();
  const { get ***REMOVED*** = useApi();
  const [admin, setAdmin] = React.useState<boolean>(false);

  const getAuth = async () => {
    const response = await get("/admin/isAdmin", {
      headers: { "HRD-token": token ***REMOVED***,
***REMOVED***);
    if (!!response && response.data.msgCode === 200) {
      setAdmin(response.data.t);
***REMOVED***
  ***REMOVED***;
  useEffect(() => {
    if (!!token) {
      getAuth();
***REMOVED***
  ***REMOVED***, [token]);
  return (
    <>
      <VStack spacing={0***REMOVED*** bg={"white"***REMOVED*** h={"calc(100%-4rem)"***REMOVED*** w={"15vw"***REMOVED***>
        <Button
          leftIcon={<AiOutlineHome />***REMOVED***
          colorScheme="teal"
          variant="ghost"
          padding={6***REMOVED***
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(0);
  ***REMOVED******REMOVED***
        >
          My Profile
        </Button>
        <Button
          leftIcon={<AiOutlineTag />***REMOVED***
          colorScheme="teal"
          variant="ghost"
          padding={6***REMOVED***
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(1);
  ***REMOVED******REMOVED***
        >
          Tag Settings
        </Button>
        <Button
          leftIcon={<RiGalleryLine />***REMOVED***
          colorScheme="teal"
          variant="ghost"
          padding={6***REMOVED***
          minW="100%"
          justifyContent="flex-start"
          onClick={() => {
            changeSelection(2);
  ***REMOVED******REMOVED***
        >
          Gallery Settings
        </Button>
    ***REMOVED***admin ? (
          <>
            <Button
              leftIcon={<RiFileSettingsLine />***REMOVED***
              colorScheme="teal"
              variant="ghost"
              padding={6***REMOVED***
              minW="100%"
              justifyContent="flex-start"
              onClick={() => {
                changeSelection(3);
 ***REMOVED*****REMOVED******REMOVED***
            >
              Admin Settings
            </Button>
            <Button
              leftIcon={<AiOutlineUser />***REMOVED***
              colorScheme="teal"
              variant="ghost"
              padding={6***REMOVED***
              minW="100%"
              justifyContent="flex-start"
              onClick={() => {
                changeSelection(4);
 ***REMOVED*****REMOVED******REMOVED***
            >
              Admin User Settings
            </Button>
          </>
        ) : (
          ""
        )***REMOVED***
      </VStack>
    </>
  );
***REMOVED***;
export default NavManager;
