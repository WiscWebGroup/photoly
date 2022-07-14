import { Stack } from "@chakra-ui/react";
import React, { useEffect, useState, useRef } from "react";
import useToken from "../../hooks/useToken";
import useApi from "../../hooks/useApi";
import Navbar from "../../components/Navbar";
import NavManager from "../../components/manage/NavManager";
import MyProfile from "../../components/manage/MyProfile";

const Manage: React.FC = () => {
  const token = useToken();
  const { get, post } = useApi();
  const [currentSelected, setCurrentSelected] = useState<number>(0);

  const changeSelection = (num: number) => {
    setCurrentSelected(num);
  };

  return (
    <>
      <Navbar />

      <Stack
        bg={"gray.50"}
        h={"calc(100% - 4rem)"}
        direction="row"
        w="100vw-4rem"
      >
        <NavManager changeSelection={changeSelection} />
        {currentSelected === 0 ? <MyProfile /> : ""}
      </Stack>
    </>
  );
};
export default Manage;
