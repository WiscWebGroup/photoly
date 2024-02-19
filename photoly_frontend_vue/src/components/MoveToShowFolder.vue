<script setup>
import { RouterLink, RouterView } from 'vue-router'
import router from "../router/index.js";
import axios from "axios";
import { ref, defineComponent, h, defineExpose } from 'vue';
import { useMessage, NIcon } from 'naive-ui'
import {
    Folder20Regular as Folder
} from "@vicons/fluent";

</script>

<template>
  <div>
    <!-- This part is for modal of the move-to functionality -->
    <n-modal v-model:show="showMoveToModal">
      <n-card
        style="width: 400px; border-radius: 10px;"
        title="Move to a New Location"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
      >
        <p>Currently Selected: {{ selectedNsName }}</p>

        <n-tree
            block-line
            expand-on-click
            :data="selectedOptionF"
            :on-update:selected-keys="handleUpdateSelection"
            :on-update:expanded-keys="handleExpansion"
        />

        <n-space>
            <n-button type="primary" secondary round size="large" style="margin-top: 1rem; width: 100%;" @click="doUpdate">
                Update
            </n-button>
            <n-button round size="large" style="margin-top: 1rem; width: 100%;" @click="() => {this.showMoveToModal = false;}">
                Cancel
            </n-button>
        </n-space>

      </n-card>
    </n-modal>
    <!-- ENDOF: This part is for modal of the move-to functionality -->
  </div>
</template>

<script>
export default defineComponent({
  data() {
    return {
      rootNsId: -1,
      selectedNsId: -1,
      selectedNsName: "",
      showMoveToModal: false,
      selectedOptionF: [

      ],
    }
  },
  setup () {
    return {

    }
  },
  emits: ["moveToUpdate"],
  expose: ["openModal"],
  props: {
    originalNsId: Number,
    disableOrgNsId: Boolean,
  },
  methods: {
    openModal () {
        this.showMoveToModal = true;
    },
    doUpdate () {
        this.$emit('moveToUpdate', this.selectedNsId);
        this.showMoveToModal = false;
    },
    handleExpansion (_keys, _option, meta) {
        if (meta.action !== "collapse")
        {
            // console.log(_keys, _option, meta)
            var nsId = _keys[_keys.length - 1];
            this.getChildren(nsId);
        }
    },
    handleUpdateSelection (nsId, nsObj) {
        if (nsId.length === 1)
        {
            this.selectedNsId = nsId[0];
            this.selectedNsName = nsObj[0].label;
        }        
        //console.log(nsId, nsObj)
    },
    getRoot () {
        axios({
          method: 'get',
          baseURL: '',
          url: import.meta.env.VITE_APP_BASE_URL + "/namespace/getRoot",
          headers: {
            "HRD-Token": localStorage.getItem("HRD-Token")
          },
          data: {
        }
        }).then((response) => {
          if (response.data.msgCode === 200)
          {
            var dataT = response.data.t;
            this.rootNsId = dataT.nsId;
            this.selectedOptionF.push({
                "label": "root",
                "key": dataT.nsId,
                "prefix": () => h(NIcon, null, {
                    default: () => h(Folder)
                }),
                "children": [],
            });
            
          }else {
            window.$message.warning("Load Root Error!")
          }
        })
        .catch(function (error) { // 请求失败处理
          window.$message.error(error);
        });
    },
    getChildrenHelper (folderInfo, parentId) {
        if (this.disableOrgNsId && folderInfo["key"] === this.originalNsId)
        {

        }else {
            if (folderInfo["key"] === parentId)
            {
                axios({
                    method: 'get',
                    baseURL: '',
                    url: import.meta.env.VITE_APP_BASE_URL + "/namespace/getChildren?parentId=" + parentId,
                    headers: {
                        "HRD-Token": localStorage.getItem("HRD-Token")
                    },
                    data: {
                    }
                }).then((response) => {
                    if (response.data.msgCode === 200)
                    {
                        var dataT = response.data.t;
                        folderInfo["children"] = [];
                        dataT.forEach((folder) => {
                            if (this.disableOrgNsId && folder.nsId === this.originalNsId)
                            {
                                folderInfo["children"].push({
                                    "label": folder.nsName,
                                    "key": folder.nsId,
                                    "prefix": () => h(NIcon, null, {
                                        default: () => h(Folder)
                                    }),
                                    "children": [],
                                    "disabled": true
                                });
                            }else {
                                folderInfo["children"].push({
                                    "label": folder.nsName,
                                    "key": folder.nsId,
                                    "prefix": () => h(NIcon, null, {
                                        default: () => h(Folder)
                                    }),
                                    "children": [],
                                });
                            }
                            
                        });
                        return;
                    }else {
                        window.$message.warning("Query Folder Error!")
                    }
                })
                .catch(function (error) { // 请求失败处理
                    window.$message.error(error);
                });
            }else {
                // BFS Search
                if ((!"children" in folderInfo) || folderInfo["children"].length === 0)
                {
                    // stop
                }else {
                    folderInfo["children"].forEach((folder) => {
                        this.getChildrenHelper(folder, parentId);
                    })
                }
            }
        }
        
    },
    getChildren (parentId) {
        this.selectedOptionF.forEach((folder) => {
            this.getChildrenHelper(folder, parentId);
        })
    },
  },
  computed: {

  },
  mounted () {
    window.$message = useMessage();
    this.getRoot();
  },
  components: {

  }
})
</script>

<style scoped>

</style>
