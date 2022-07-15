import {Stack***REMOVED*** from "@chakra-ui/react";
import React, {useState***REMOVED*** from "react";
import useToken from "../../hooks/useToken";
import Navbar from "../../components/Navbar";
import NavManager from "../../components/manage/NavManager";
import MyProfile from "../../components/manage/MyProfile";
import TagSetting from "../../components/manage/TagSetting";
import AdminSettings from "../../components/manage/AdminSettings";
import GallerySetting from "../../components/manage/GallerySetting";

const Manage: React.FC = () => {
    useToken();
    const [currentSelected, setCurrentSelected] = useState<number>(0);

  const changeSelection = (num: number) => {
    setCurrentSelected(num);
  ***REMOVED***;

  return (
    <>
      <Navbar />

      <Stack
        bg={"gray.50"***REMOVED***
        h={"calc(100% - 4rem)"***REMOVED***
        direction="row"
        w="100vw-4rem"
      >
        <NavManager changeSelection={changeSelection***REMOVED*** />
    ***REMOVED***currentSelected === 0 ? <MyProfile /> : ""***REMOVED***
    ***REMOVED***currentSelected === 1 ? <TagSetting /> : ""***REMOVED***
    ***REMOVED***currentSelected === 2 ? <GallerySetting /> : ""***REMOVED***
    ***REMOVED***currentSelected === 3 ? <AdminSettings /> : ""***REMOVED***
      </Stack>
    </>
  );
***REMOVED***;
export default Manage;
